//go:build darwin || linux

package target

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"bananas-pos/internal/job"
)

type commandRunner interface {
	Run(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	return cmd.CombinedOutput()
}

type RawSpool struct {
	runner commandRunner
}

func NewRawSpool() *RawSpool {
	return &RawSpool{runner: execRunner{}}
}

func (r *RawSpool) Name() string {
	return "system-print-queue"
}

func (r *RawSpool) Send(ctx context.Context, printJob job.PrintJob) error {
	if len(printJob.Raw) == 0 {
		return errors.New("print job payload is empty")
	}

	args := []string{"-o", "raw"}
	if title := spoolTitle(printJob); title != "" {
		args = append(args, "-t", title)
	}

	output, err := r.runner.Run(ctx, "lp", args, printJob.Raw)
	if err != nil {
		return fmt.Errorf("submit to system print queue: %w", commandError(err, output))
	}

	return nil
}

func (r *RawSpool) Health(ctx context.Context) error {
	if _, err := r.runner.Run(ctx, "lpstat", []string{"-r"}, nil); err != nil {
		return fmt.Errorf("check print scheduler: %w", err)
	}

	output, err := r.runner.Run(ctx, "lpstat", []string{"-d"}, nil)
	if err != nil {
		return fmt.Errorf("check default printer: %w", commandError(err, output))
	}

	return nil
}

func spoolTitle(printJob job.PrintJob) string {
	if strings.TrimSpace(printJob.ID) != "" {
		return printJob.ID
	}
	if strings.TrimSpace(printJob.Source) != "" {
		return "bananas-pos-" + printJob.Source
	}
	return "bananas-pos"
}

func commandError(err error, output []byte) error {
	message := strings.TrimSpace(string(output))
	if message == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, message)
}
