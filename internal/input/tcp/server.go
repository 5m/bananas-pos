package tcpinput

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bananas-pos/internal/job"
	"bananas-pos/internal/target"
)

type Server struct {
	addr   string
	target target.Target

	listener net.Listener
	jobSeq   atomic.Uint64
	wg       sync.WaitGroup
}

func NewServer(addr string, target target.Target) *Server {
	return &Server{
		addr:   addr,
		target: target,
	}
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	s.listener = listener
	log.Printf("tcp input listening on %s", s.addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.listener == nil {
		return nil
	}

	err := s.listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.wg.Wait()
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	if err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	log.Printf("tcp connection from %s", conn.RemoteAddr())

	raw, err := io.ReadAll(conn)
	if err != nil {
		log.Printf("tcp read failed from %s: %v", conn.RemoteAddr(), err)
		return
	}

	labels := splitLabels(string(raw))
	if len(labels) == 0 {
		labels = []string{string(raw)}
	}

	createdAt := time.Now()
	for _, label := range labels {
		printJob := job.PrintJob{
			ID:          fmt.Sprintf("tcp-%d", s.jobSeq.Add(1)),
			Raw:         []byte(label),
			ContentType: "application/zpl",
			Source:      "tcp",
			CreatedAt:   createdAt,
		}

		if err := s.target.Send(context.Background(), printJob); err != nil {
			log.Printf("tcp print job failed: %v", err)
			_, _ = io.WriteString(conn, "ERROR: "+err.Error()+"\n")
			return
		}
	}

	_, _ = io.WriteString(conn, "OK\n")
}

func splitLabels(raw string) []string {
	var labels []string
	searchFrom := 0

	for {
		start := strings.Index(raw[searchFrom:], "^XA")
		if start < 0 {
			return labels
		}
		start += searchFrom

		end := strings.Index(raw[start:], "^XZ")
		if end < 0 {
			return labels
		}
		end += start + len("^XZ")

		labels = append(labels, raw[start:end])
		searchFrom = end
	}
}
