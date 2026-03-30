package httpinput

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"bananas-pos/internal/input"
	"bananas-pos/internal/job"
	"bananas-pos/internal/meta"
	"bananas-pos/internal/target"
)

type Server struct {
	addr      string
	target    target.Target
	server    *http.Server
	jobSeq    atomic.Uint64
	startedAt time.Time
}

func NewServer(addr string, outputTarget target.Target) *Server {
	s := &Server{
		addr:      addr,
		target:    outputTarget,
		startedAt: time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/_/health", s.handleHealth)
	mux.HandleFunc("/zpl", s.handlePrint)
	if routeTarget, ok := outputTarget.(target.HTTPRoutes); ok {
		routeTarget.RegisterRoutes(mux)
	}
	mux.HandleFunc("/", s.handleRoot)

	s.server = &http.Server{
		Addr:    addr,
		Handler: loggingMiddleware(corsMiddleware(mux)),
	}

	return s
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) Start() error {
	log.Printf("http input listening on %s", s.addr)
	err := s.server.ListenAndServe()
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) handleHealth(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := s.target.Health(req.Context())
	status := http.StatusOK
	response := map[string]any{
		"name":    meta.AppName,
		"version": meta.Version,
		"status":  "ok",
		"target":  s.target.Name(),
	}
	if err != nil {
		status = http.StatusServiceUnavailable
		response["status"] = "error"
		response["error"] = err.Error()
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(response)
}

func (s *Server) handlePrint(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(rw, "failed to read body", http.StatusBadRequest)
		return
	}

	labels := input.SplitLabels(string(body))
	if len(labels) == 0 {
		labels = []string{string(body)}
	}

	contentType := contentTypeOrDefault(req.Header.Get("Content-Type"))
	createdAt := time.Now()
	acceptedIDs := make([]string, 0, len(labels))

	for _, label := range labels {
		printJob := job.PrintJob{
			ID:          fmt.Sprintf("http-%d", s.jobSeq.Add(1)),
			Raw:         []byte(label),
			ContentType: contentType,
			Source:      "http",
			CreatedAt:   createdAt,
		}

		if err := s.target.Send(req.Context(), printJob); err != nil {
			log.Printf("http print job failed: %v", err)
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return
		}

		acceptedIDs = append(acceptedIDs, printJob.ID)
	}

	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]any{
		"status": "accepted",
		"count":  len(acceptedIDs),
		"ids":    acceptedIDs,
		"target": s.target.Name(),
	})
}

func (s *Server) handleRoot(rw http.ResponseWriter, req *http.Request) {
	if proxyTarget, ok := s.target.(target.HTTPPassthrough); ok {
		if proxyTarget.ServeHTTPProxy(rw, req) {
			return
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]any{
		"name":      meta.AppName,
		"version":   meta.Version,
		"target":    s.target.Name(),
		"uptime":    time.Since(s.startedAt).String(),
		"endpoints": []string{"/_/health", "/zpl"},
	})
}

func contentTypeOrDefault(value string) string {
	if strings.TrimSpace(value) == "" {
		return "application/zpl"
	}
	return value
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		log.Printf("http %s %s", req.Method, req.URL.String())
		next.ServeHTTP(rw, req)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		headers := rw.Header()
		headers.Add("Vary", "Origin")
		headers.Add("Vary", "Access-Control-Request-Method")
		headers.Add("Vary", "Access-Control-Request-Headers")
		headers.Set("Access-Control-Allow-Origin", "*")
		headers.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		allowHeaders := strings.TrimSpace(req.Header.Get("Access-Control-Request-Headers"))
		if allowHeaders == "" {
			allowHeaders = "Content-Type, Authorization"
		}
		headers.Set("Access-Control-Allow-Headers", allowHeaders)

		if req.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(rw, req)
	})
}
