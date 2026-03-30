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

func TestRawSpoolSendRejectsEmptyPayload(t *testing.T) {
	target := NewRawSpool()

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
		results: []stubResult{{}, {}},
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
