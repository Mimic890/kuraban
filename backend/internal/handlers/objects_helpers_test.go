package handlers

import (
	"strings"
	"testing"
)

func TestSafeContentType_RewritesExecutableTypes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"text/html", "application/octet-stream"},
		{"text/html; charset=utf-8", "application/octet-stream"},
		{"TEXT/HTML", "application/octet-stream"},
		{"  text/html  ", "application/octet-stream"},
		{"application/xhtml+xml", "application/octet-stream"},
		{"image/svg+xml", "application/octet-stream"},
		{"application/xml", "application/octet-stream"},
		{"text/xml", "application/octet-stream"},
		{"application/javascript", "application/octet-stream"},
		{"text/javascript", "application/octet-stream"},
		// Safe types pass through unchanged (including parameters).
		{"image/png", "image/png"},
		{"application/octet-stream", "application/octet-stream"},
		{"text/plain; charset=utf-8", "text/plain; charset=utf-8"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := safeContentType(tc.in); got != tc.want {
				t.Errorf("safeContentType(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestContentDispositionHeader_StripsPathAndEscapes(t *testing.T) {
	cases := []struct {
		name           string
		disp, key      string
		mustContain    []string
		mustNotContain []string
	}{
		{
			name: "simple filename",
			disp: "inline", key: "photo.png",
			mustContain: []string{
				`inline; filename="photo.png"; filename*=UTF-8''photo.png`,
			},
		},
		{
			name: "path components stripped",
			disp: "attachment", key: "a/b/c/file.txt",
			mustContain:    []string{`filename="file.txt"`, `filename*=UTF-8''file.txt`},
			mustNotContain: []string{`a/b/c`},
		},
		{
			name: "quote and backslash replaced in ASCII fallback",
			disp: "inline", key: `"evil\name".txt`,
			mustContain:    []string{`filename="_evil_name_.txt"`},
			mustNotContain: []string{`"evil`, `\name`},
		},
		{
			name: "control character replaced",
			disp: "inline", key: "line\nbreak.txt",
			mustContain: []string{`filename="line_break.txt"`},
		},
		{
			name: "non-ASCII preserved in filename* only",
			disp: "inline", key: "漢字.txt",
			mustContain: []string{
				`filename*=UTF-8''`,
				// fallback replaced every non-ASCII rune with _ (2 runes + .txt).
				`filename="__.txt"`,
			},
		},
		{
			name: "empty key falls back to download",
			disp: "inline", key: "",
			mustContain: []string{`filename="download"`, `filename*=UTF-8''download`},
		},
		{
			name: "key of only slashes falls back to download",
			disp: "inline", key: "/",
			mustContain: []string{`filename="download"`, `filename*=UTF-8''`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := contentDispositionHeader(tc.disp, tc.key)
			for _, sub := range tc.mustContain {
				if !strings.Contains(got, sub) {
					t.Errorf("got %q\nmust contain %q", got, sub)
				}
			}
			for _, sub := range tc.mustNotContain {
				if strings.Contains(got, sub) {
					t.Errorf("got %q\nmust NOT contain %q", got, sub)
				}
			}
		})
	}
}
