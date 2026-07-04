package store

import "regexp"

var (
	messageCredentialValueRe = regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|refresh[_-]?token|token|password|passwd|secret)\b(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`)
	messageBearerRe          = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9\-._~+/]+=*`)
	messageEmailRe           = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
)

// SanitizeMessage redacts common sensitive values from free-form message text.
func SanitizeMessage(message string) string {
	if message == "" {
		return message
	}

	sanitized := messageCredentialValueRe.ReplaceAllString(message, `$1$2[REDACTED]`)
	sanitized = messageBearerRe.ReplaceAllString(sanitized, "Bearer [REDACTED_TOKEN]")
	sanitized = messageEmailRe.ReplaceAllString(sanitized, "[REDACTED_EMAIL]")
	return sanitized
}
