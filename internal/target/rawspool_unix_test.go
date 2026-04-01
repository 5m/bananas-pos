//go:build darwin || linux

package target

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bananas-pos/internal/job"
)

type runnerCall struct {
	name  string
	args  []string
	stdin []byte
}

type stubRunner struct {
	calls   []runnerCall
	results []stubResult
}

type stubResult struct {
	output []byte
	err    error
}

func (s *stubRunner) Run(_ context.Context, name string, args []string, stdin []byte) ([]byte, error) {
	call := runnerCall{
		name:  name,
		args:  append([]string(nil), args...),
		stdin: append([]byte(nil), stdin...),
	}
	s.calls = append(s.calls, call)

	if len(s.results) == 0 {
		return nil, nil
	}

	result := s.results[0]
	s.results = s.results[1:]
	return result.output, result.err
}

func TestRawSpoolSendUsesLPWithRawOption(t *testing.T) {
	runner := &stubRunner{}
	target := &RawSpool{runner: runner}

	printJob := job.PrintJob{
		ID:     "http-7",
		Raw:    []byte("^XA^XZ"),
		Source: "http",
	}

	if err := target.Send(context.Background(), printJob); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.calls))
	}

	call := runner.calls[0]
	if call.name != "lp" {
		t.Fatalf("expected lp command, got %q", call.name)
	}
	if got := strings.Join(call.args, " "); got != "-o raw -t http-7" {
		t.Fatalf("unexpected args %q", got)
	}
	if got := string(call.stdin); got != "^XA^XZ" {
		t.Fatalf("unexpected stdin %q", got)
	}
}

func TestRawSpoolSendUsesConfiguredPrinter(t *testing.T) {
	runner := &stubRunner{}
	target := &RawSpool{runner: runner, printerName: "Kitchen"}

	if err := target.Send(context.Background(), job.PrintJob{Raw: []byte("^XA^XZ")}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.calls))
	}
	if got := strings.Join(runner.calls[0].args, " "); got != "-o raw -d Kitchen -t bananas-pos" {
		t.Fatalf("unexpected args %q", got)
	}
}

func TestRawSpoolSendRejectsEmptyPayload(t *testing.T) {
	target := NewRawSpool("")

	err := target.Send(context.Background(), job.PrintJob{})
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestRawSpoolSendIncludesCommandOutputInError(t *testing.T) {
	runner := &stubRunner{
		results: []stubResult{{
			output: []byte("lp: printer offline"),
			err:    errors.New("exit status 1"),
		}},
	}
	target := &RawSpool{runner: runner}

	err := target.Send(context.Background(), job.PrintJob{Raw: []byte("^XA^XZ")})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "printer offline") {
		t.Fatalf("expected stderr in error, got %q", err)
	}
}

func TestRawSpoolHealthChecksSchedulerAndDefaultPrinter(t *testing.T) {
	runner := &stubRunner{
		results: []stubResult{{}, {output: []byte("system default destination: SAM4S\n")}},
	}
	target := &RawSpool{runner: runner}

	if err := target.Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(runner.calls))
	}

	first := runner.calls[0]
	if first.name != "lpstat" || strings.Join(first.args, " ") != "-r" {
		t.Fatalf("unexpected first command: %s %s", first.name, strings.Join(first.args, " "))
	}

	second := runner.calls[1]
	if second.name != "lpstat" || strings.Join(second.args, " ") != "-d" {
		t.Fatalf("unexpected second command: %s %s", second.name, strings.Join(second.args, " "))
	}
}

func TestRawSpoolHealthReturnsDefaultPrinterError(t *testing.T) {
	runner := &stubRunner{
		results: []stubResult{
			{},
			{
				output: []byte("no system default destination"),
				err:    errors.New("exit status 1"),
			},
		},
	}
	target := &RawSpool{runner: runner}

	err := target.Health(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "default printer") {
		t.Fatalf("expected default printer error, got %q", err)
	}
	if !strings.Contains(err.Error(), "no system default destination") {
		t.Fatalf("expected lpstat output in error, got %q", err)
	}
}

func TestRawSpoolHealthRejectsUnexpectedDefaultPrinterOutput(t *testing.T) {
	runner := &stubRunner{
		results: []stubResult{
			{},
			{output: []byte("printer status unavailable")},
		},
	}
	target := &RawSpool{runner: runner}

	err := target.Health(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unexpected default printer output") {
		t.Fatalf("expected parse error, got %q", err)
	}
}

func TestRawSpoolDescriptionReturnsDefaultPrinterName(t *testing.T) {
	runner := &stubRunner{
		results: []stubResult{
			{output: []byte("system default destination: SAM4S\n")},
		},
	}
	target := &RawSpool{runner: runner}

	name, err := target.Description(context.Background())
	if err != nil {
		t.Fatalf("Description() error = %v", err)
	}
	if name != "SAM4S" {
		t.Fatalf("expected printer name SAM4S, got %q", name)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call.name != "lpstat" || strings.Join(call.args, " ") != "-d" {
		t.Fatalf("unexpected command: %s %s", call.name, strings.Join(call.args, " "))
	}
}

func TestRawSpoolDescriptionReturnsConfiguredPrinterName(t *testing.T) {
	target := &RawSpool{printerName: "Kitchen"}

	name, err := target.Description(context.Background())
	if err != nil {
		t.Fatalf("Description() error = %v", err)
	}
	if name != "Kitchen" {
		t.Fatalf("expected printer name Kitchen, got %q", name)
	}
}

func TestRawSpoolAvailablePrintersUsesLPStat(t *testing.T) {
	runner := &stubRunner{
		results: []stubResult{{
			output: []byte("Receipt\nKitchen\nReceipt\n\n"),
		}},
	}
	target := &RawSpool{runner: runner}

	printers, err := target.AvailablePrinters(context.Background())
	if err != nil {
		t.Fatalf("AvailablePrinters() error = %v", err)
	}
	if got := strings.Join(printers, ","); got != "Receipt,Kitchen" {
		t.Fatalf("unexpected printers %q", got)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.calls))
	}
	if call := runner.calls[0]; call.name != "lpstat" || strings.Join(call.args, " ") != "-e" {
		t.Fatalf("unexpected command: %s %s", call.name, strings.Join(call.args, " "))
	}
}
