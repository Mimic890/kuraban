package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestFromCtx_ReturnsBoundLogger(t *testing.T) {
	var buf bytes.Buffer
	l := zerolog.New(&buf).With().Str("request_id", "req-1").Logger()

	ctx := IntoCtx(context.Background(), l)
	got := FromCtx(ctx)
	got.Info().Msg("hello")

	var parsed map[string]any
	line := strings.TrimSpace(buf.String())
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("not JSON: %v — %s", err, line)
	}
	if parsed["request_id"] != "req-1" {
		t.Errorf("request_id = %v, want req-1", parsed["request_id"])
	}
	if parsed["message"] != "hello" {
		t.Errorf("message = %v, want hello", parsed["message"])
	}
}

func TestFromCtx_NoLoggerFallsBackToDisabled(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("FromCtx panicked: %v", r)
		}
	}()
	l := FromCtx(context.Background())
	l.Info().Msg("should not panic")
}
