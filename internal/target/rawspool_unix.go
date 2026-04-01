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
	runner      commandRunner
	printerName string
}

func NewRawSpool(printerName string) *RawSpool {
	return &RawSpool{runner: execRunner{}, printerName: strings.TrimSpace(printerName)}
}

func (r *RawSpool) Name() string {
	return "system-print-queue"
}

func (r *RawSpool) Send(ctx context.Context, printJob job.PrintJob) error {
	if len(printJob.Raw) == 0 {
		return errors.New("print job payload is empty")
	}

	args := []string{"-o", "raw"}
	if r.printerName != "" {
		args = append(args, "-d", r.printerName)
	}
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

	if r.printerName != "" {
		output, err := r.runner.Run(ctx, "lpstat", []string{"-p", r.printerName}, nil)
		if err != nil {
			return fmt.Errorf("check configured printer: %w", commandError(err, output))
		}
		return nil
	}

	output, err := r.runner.Run(ctx, "lpstat", []string{"-d"}, nil)
	if err != nil {
		return fmt.Errorf("check default printer: %w", commandError(err, output))
	}
	if _, err := parseDefaultDestination(output); err != nil {
		return fmt.Errorf("check default printer: %w", err)
	}

	return nil
}

func (r *RawSpool) Description(ctx context.Context) (string, error) {
	if r.printerName != "" {
		return r.printerName, nil
	}

	output, err := r.runner.Run(ctx, "lpstat", []string{"-d"}, nil)
	if err != nil {
		return "", fmt.Errorf("resolve default printer: %w", commandError(err, output))
	}

	name, err := parseDefaultDestination(output)
	if err != nil {
		return "", fmt.Errorf("resolve default printer: %w", err)
	}

	return name, nil
}

func (r *RawSpool) AvailablePrinters(ctx context.Context) ([]string, error) {
	output, err := r.runner.Run(ctx, "lpstat", []string{"-e"}, nil)
	if err != nil {
		return nil, fmt.Errorf("list printers: %w", commandError(err, output))
	}
	return parsePrinterList(output), nil
}

func parseDefaultDestination(output []byte) (string, error) {
	message := strings.TrimSpace(string(output))
	if message == "" {
		return "", errors.New("default printer output is empty")
	}

	const prefix = "system default destination:"
	if strings.HasPrefix(message, prefix) {
		name := strings.TrimSpace(strings.TrimPrefix(message, prefix))
		if name == "" {
			return "", errors.New("default printer name is empty")
		}
		return name, nil
	}

	return "", fmt.Errorf("unexpected default printer output: %s", message)
}

func parsePrinterList(output []byte) []string {
	lines := strings.Split(string(output), "\n")
	printers := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		printers = append(printers, name)
	}
	return printers
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
