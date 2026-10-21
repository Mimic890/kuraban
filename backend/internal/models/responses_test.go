package models

import "testing"

func TestSuccessResponse(t *testing.T) {
	payload := map[string]int{"count": 3}
	r := SuccessResponse(payload)
	if !r.Success {
		t.Error("Success should be true")
	}
	if r.Error != nil {
		t.Errorf("Error should be nil, got %+v", r.Error)
	}
	m, ok := r.Data.(map[string]int)
	if !ok {
		t.Fatalf("Data type = %T, want map[string]int", r.Data)
	}
	if m["count"] != 3 {
		t.Errorf("Data.count = %d, want 3", m["count"])
	}
}

func TestSuccessResponse_NilData(t *testing.T) {
	r := SuccessResponse(nil)
	if !r.Success {
		t.Error("Success should be true even with nil data")
	}
	if r.Data != nil {
		t.Errorf("Data = %v, want nil", r.Data)
	}
}

func TestErrorResponse(t *testing.T) {
	r := ErrorResponse(ErrCodeBadRequest, "bad input")
	if r.Success {
		t.Error("Success should be false for error response")
	}
	if r.Data != nil {
		t.Errorf("Data should be nil, got %v", r.Data)
	}
	if r.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if r.Error.Code != ErrCodeBadRequest || r.Error.Message != "bad input" {
		t.Errorf("Error = %+v", r.Error)
	}
}
