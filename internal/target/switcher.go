package target

import (
	"context"
	"net/http"
	"sync"

	"bananas-pos/internal/job"
)

type Switcher struct {
	mu      sync.RWMutex
	current Target
}

func NewSwitcher(initial Target) *Switcher {
	return &Switcher{current: initial}
}

func (s *Switcher) Name() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current == nil {
		return ""
	}
	return s.current.Name()
}

func (s *Switcher) Send(ctx context.Context, printJob job.PrintJob) error {
	s.mu.RLock()
	current := s.current
	s.mu.RUnlock()
	return current.Send(ctx, printJob)
}

func (s *Switcher) Health(ctx context.Context) error {
	s.mu.RLock()
	current := s.current
	s.mu.RUnlock()
	return current.Health(ctx)
}

func (s *Switcher) Set(next Target) error {
	s.mu.Lock()
	prev := s.current
	s.current = next
	s.mu.Unlock()

	if shutdownTarget, ok := prev.(Shutdowner); ok {
		return shutdownTarget.Shutdown()
	}
	return nil
}

func (s *Switcher) Current() Target {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *Switcher) Shutdown() error {
	s.mu.RLock()
	current := s.current
	s.mu.RUnlock()
	if shutdownTarget, ok := current.(Shutdowner); ok {
		return shutdownTarget.Shutdown()
	}
	return nil
}

func (s *Switcher) Start() error {
	s.mu.RLock()
	current := s.current
	s.mu.RUnlock()
	if starter, ok := current.(Starter); ok {
		return starter.Start()
	}
	return nil
}

func (s *Switcher) ShowWindow() {
	s.mu.RLock()
	current := s.current
	s.mu.RUnlock()
	if presenter, ok := current.(WindowPresenter); ok {
		presenter.ShowWindow()
	}
}

func (s *Switcher) RegisterRoutes(mux *http.ServeMux) {
	s.mu.RLock()
	current := s.current
	s.mu.RUnlock()
	if routeTarget, ok := current.(HTTPRoutes); ok {
		routeTarget.RegisterRoutes(mux)
	}
}

func (s *Switcher) ServeHTTPProxy(rw http.ResponseWriter, req *http.Request) bool {
	s.mu.RLock()
	current := s.current
	s.mu.RUnlock()
	if proxyTarget, ok := current.(HTTPPassthrough); ok {
		return proxyTarget.ServeHTTPProxy(rw, req)
	}
	return false
}
