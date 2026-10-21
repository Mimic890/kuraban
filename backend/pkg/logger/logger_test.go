package logger

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
)

// serializeLoggerTests guards the global mutations (os.Stdout, globalLogger,
// zerolog global). These tests cannot run in parallel with each other.
var serializeLoggerTests sync.Mutex

// captureStdout swaps os.Stdout for a pipe, calls fn, restores stdout and the
// zerolog global level (Init changes it), and returns everything written
// during fn.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	old := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })
	oldLevel := zerolog.GlobalLevel()
	t.Cleanup(func() { zerolog.SetGlobalLevel(oldLevel) })

	// Run fn and close writer so the reader unblocks.
	doneWrite := make(chan struct{})
	go func() {
		fn()
		_ = w.Close()
		close(doneWrite)
	}()

	var buf strings.Builder
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		buf.WriteString(scanner.Text())
		buf.WriteByte('\n')
	}
	// Drain any residual (shouldn't happen after Close, but safe):
	_, _ = io.Copy(io.Discard, r)
	<-doneWrite
	return buf.String()
}

func TestInit_JSONFormatProducesParseableOutput(t *testing.T) {
	serializeLoggerTests.Lock()
	defer serializeLoggerTests.Unlock()

	out := captureStdout(t, func() {
		Init(Config{Level: "info", Format: "json"})
		Info().Str("user", "alice").Msg("hello")
	})

	// Find the first non-empty line; parse as JSON.
	var line string
	for l := range strings.SplitSeq(out, "\n") {
		if strings.TrimSpace(l) != "" {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("no log output captured; stdout = %q", out)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nline: %s", err, line)
	}

	// Field assertions — zerolog uses "message" for the msg and "level" for level.
	if got, _ := parsed["message"].(string); got != "hello" {
		t.Errorf("message = %v, want hello", parsed["message"])
	}
	if got, _ := parsed["user"].(string); got != "alice" {
		t.Errorf("user field = %v, want alice", parsed["user"])
	}
	if got, _ := parsed["level"].(string); got != "info" {
		t.Errorf("level = %v, want info", parsed["level"])
	}
	if _, ok := parsed["time"]; !ok {
		t.Errorf("expected time field; got keys %v", keysOf(parsed))
	}
	if _, ok := parsed["caller"]; !ok {
		t.Errorf("expected caller field; got keys %v", keysOf(parsed))
	}
}

func TestInit_LevelFilterDropsBelowThreshold(t *testing.T) {
	serializeLoggerTests.Lock()
	defer serializeLoggerTests.Unlock()

	out := captureStdout(t, func() {
		Init(Config{Level: "warn", Format: "json"})
		Debug().Msg("debug-dropped")
		Info().Msg("info-dropped")
		Warn().Msg("warn-kept")
		Error().Msg("error-kept")
	})

	if strings.Contains(out, "debug-dropped") {
		t.Errorf("debug event leaked through warn filter: %s", out)
	}
	if strings.Contains(out, "info-dropped") {
		t.Errorf("info event leaked through warn filter: %s", out)
	}
	if !strings.Contains(out, "warn-kept") {
		t.Errorf("warn event missing: %s", out)
	}
	if !strings.Contains(out, "error-kept") {
		t.Errorf("error event missing: %s", out)
	}
}

func TestInit_UnknownLevelDefaultsToInfo(t *testing.T) {
	serializeLoggerTests.Lock()
	defer serializeLoggerTests.Unlock()

	out := captureStdout(t, func() {
		Init(Config{Level: "gibberish", Format: "json"})
		Debug().Msg("debug-should-be-dropped")
		Info().Msg("info-should-appear")
	})

	if strings.Contains(out, "debug-should-be-dropped") {
		t.Errorf("debug leaked at default info level: %s", out)
	}
	if !strings.Contains(out, "info-should-appear") {
		t.Errorf("info missing at default info level: %s", out)
	}
}

func TestInit_TextFormatDoesNotCrashAndIsNotJSON(t *testing.T) {
	serializeLoggerTests.Lock()
	defer serializeLoggerTests.Unlock()

	out := captureStdout(t, func() {
		Init(Config{Level: "info", Format: "text"})
		Info().Str("k", "v").Msg("plain")
	})

	if !strings.Contains(out, "plain") {
		t.Errorf("text output missing message: %s", out)
	}
	// Console writer output is ANSI-colored key=value form, not JSON.
	var parsed map[string]any
	if json.Unmarshal([]byte(strings.Split(out, "\n")[0]), &parsed) == nil {
		t.Errorf("text format unexpectedly parsed as JSON: %s", out)
	}
}

func TestGet_AutoInitializesWhenUnused(t *testing.T) {
	serializeLoggerTests.Lock()
	defer serializeLoggerTests.Unlock()

	// Forcibly clear the global so Get() hits the lazy-init branch.
	globalLogger = nil

	l := Get()
	if l == nil {
		t.Fatal("Get() returned nil; lazy init did not run")
	}
	if globalLogger == nil {
		t.Fatal("globalLogger still nil after Get()")
	}
}

func TestWithError_AttachesErrorField(t *testing.T) {
	serializeLoggerTests.Lock()
	defer serializeLoggerTests.Unlock()

	out := captureStdout(t, func() {
		Init(Config{Level: "error", Format: "json"})
		WithError(io.EOF).Msg("boom")
	})

	line := firstNonEmptyLine(out)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("not JSON: %v — %s", err, line)
	}
	if got, _ := parsed["error"].(string); got != io.EOF.Error() {
		t.Errorf("error field = %v, want %q", parsed["error"], io.EOF.Error())
	}
	if got, _ := parsed["level"].(string); got != "error" {
		t.Errorf("level = %v, want error", parsed["level"])
	}
}

// --- helpers ---

func firstNonEmptyLine(s string) string {
	for l := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(l) != "" {
			return l
		}
	}
	return ""
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
