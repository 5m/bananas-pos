package target

import (
	"context"
	"errors"
	"net/http"

	"bananas-printer/internal/job"
)

var ErrNotImplemented = errors.New("target mode not implemented")

type Target interface {
	Name() string
	Send(ctx context.Context, job job.PrintJob) error
	Health(ctx context.Context) error
}

type HTTPPassthrough interface {
	ServeHTTPProxy(rw http.ResponseWriter, req *http.Request) bool
}

type HTTPRoutes interface {
	RegisterRoutes(mux *http.ServeMux)
}

type Shutdowner interface {
	Shutdown() error
}

type Starter interface {
	Start() error
}

type WindowPresenter interface {
	ShowWindow()
}
