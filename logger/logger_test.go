package logger

import (
	"io"
	"os"
	"testing"

	"github.com/fatih/color"
)

func TestSetSilentSuppressesNonDefault(t *testing.T) {
	originalNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() {
		color.NoColor = originalNoColor
	})

	SetSilent(true)
	t.Cleanup(func() {
		SetSilent(false)
	})

	stdout := captureStdout(t, func() {
		Info("hidden\n")
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty when silent", stdout)
	}

	stdout = captureStdout(t, func() {
		Default("visible\n")
	})
	if stdout != "visible\n" {
		t.Fatalf("stdout = %q, want %q for default log", stdout, "visible\n")
	}

	stderr := captureStderr(t, func() {
		Error("hidden\n")
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty when silent", stderr)
	}

	SetSilent(false)
	stdout = captureStdout(t, func() {
		Info("shown\n")
	})
	if stdout != "[INFO] shown\n" {
		t.Fatalf("stdout = %q, want %q when silent disabled", stdout, "[INFO] shown\n")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return captureStream(t, &os.Stdout, fn)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return captureStream(t, &os.Stderr, fn)
}

func captureStream(t *testing.T, stream **os.File, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	defer r.Close()

	original := *stream
	*stream = w
	defer func() {
		*stream = original
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("closing writer: %v", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading pipe: %v", err)
	}

	return string(data)
}
