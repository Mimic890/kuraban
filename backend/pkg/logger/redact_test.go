package logger

import "testing"

func TestRedactKey(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "***"},
		{"short", "abc", "***"},
		{"just-under-threshold", "abcdefghijk", "***"}, // 11 chars
		{"at-threshold", "abcdefghijkl", "abcd…ijkl"},  // 12 chars
		{"long", "GK5a9bfcdefghijklzW9q", "GK5a…zW9q"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RedactKey(tc.in); got != tc.want {
				t.Errorf("RedactKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRedactToken_AlwaysStars(t *testing.T) {
	for _, in := range []string{"", "abc", "very-long-secret-token-value"} {
		if got := RedactToken(in); got != "***" {
			t.Errorf("RedactToken(%q) = %q, want ***", in, got)
		}
	}
}
