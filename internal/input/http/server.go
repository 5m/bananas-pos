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

	"bananas-printer/internal/job"
	"bananas-printer/internal/target"
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
		Handler: loggingMiddleware(mux),
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
		"status": "ok",
		"target": s.target.Name(),
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

	job := job.PrintJob{
		ID:          fmt.Sprintf("http-%d", s.jobSeq.Add(1)),
		Raw:         body,
		ContentType: contentTypeOrDefault(req.Header.Get("Content-Type")),
		Source:      "http",
		CreatedAt:   time.Now(),
	}

	if err := s.target.Send(req.Context(), job); err != nil {
		log.Printf("http print job failed: %v", err)
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]string{
		"status": "accepted",
		"id":     job.ID,
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
		"name":      "Bananas Printer",
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
