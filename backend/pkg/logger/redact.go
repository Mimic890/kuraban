package logger

// redactThreshold is the minimum length at which partial visibility is shown.
// Below this, the entire value is replaced with "***" because first-4/last-4
// would leak too much of short strings.
const redactThreshold = 12

// RedactKey returns a partially-visible form of a non-secret identifier
// (access key ID, user ID, etc.) showing first 4 and last 4 characters.
// Shorter values are fully redacted to avoid over-exposure.
func RedactKey(s string) string {
	if len(s) < redactThreshold {
		return "***"
	}
	return s[:4] + "…" + s[len(s)-4:]
}

// RedactToken returns "***" for any secret (passwords, bearer tokens, JWT,
// client secrets). Secrets must never be partially visible in logs.
func RedactToken(s string) string {
	return "***"
}
