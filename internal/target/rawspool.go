package target

import (
	"context"

	"bananas-printer/internal/job"
)

type RawSpool struct{}

func NewRawSpool() *RawSpool {
	return &RawSpool{}
}

func (r *RawSpool) Name() string {
	return "system-print-queue"
}

func (r *RawSpool) Send(context.Context, job.PrintJob) error {
	return ErrNotImplemented
}

func (r *RawSpool) Health(context.Context) error {
	return ErrNotImplemented
}
