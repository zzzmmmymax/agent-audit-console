package redact

import (
	"regexp"
	"strings"
)

type rule struct {
	pattern     *regexp.Regexp
	replacement string
}

type Redactor struct{ rules []rule }

func New() *Redactor {
	return &Redactor{rules: []rule{
		{regexp.MustCompile(`(?is)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`), `[REDACTED PRIVATE KEY]`},
		{regexp.MustCompile(`(?i)(authorization\s*:\s*(?:bearer|basic)\s+)([^\s]+)`), `$1[REDACTED]`},
		{regexp.MustCompile(`(?i)(bearer\s+)([A-Za-z0-9._~+/=-]{8,})`), `$1[REDACTED]`},
		{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`), `[REDACTED]`},
		{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`), `[REDACTED]`},
		{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`), `[REDACTED]`},
		{regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), `[REDACTED]`},
		{regexp.MustCompile(`(?i)(token|password|passwd|pwd|api[_-]?key|secret|github[_-]?token|openai[_-]?key|aws[_-]?(?:secret|access)[_-]?key|connection[_-]?string)(\s*[:=]\s*)([^\s,;&]+)`), `$1$2[REDACTED]`},
		{regexp.MustCompile(`(?i)(--?(?:token|password|passwd|pwd|api[_-]?key|secret|github[_-]?token|openai[_-]?key|aws[_-]?(?:secret|access)[_-]?key)\s+)([^\s]+)`), `$1[REDACTED]`},
		{regexp.MustCompile(`(?i)("(?:token|password|passwd|pwd|api[_-]?key|secret|authorization|github[_-]?token|openai[_-]?key|aws[_-]?(?:secret|access)[_-]?key|connection[_-]?string)"\s*:\s*")([^"]+)`), `$1[REDACTED]`},
		{regexp.MustCompile(`(?i)(postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis)://([^\s/@:]+):([^\s/@]+)@`), `$1://[REDACTED]@[REDACTED]`},
	}}
}

func (r *Redactor) String(value string) string {
	for _, item := range r.rules {
		value = item.pattern.ReplaceAllString(value, item.replacement)
	}
	return value
}

func (r *Redactor) Strings(values []string) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = r.String(value)
	}
	return result
}

func (r *Redactor) Value(value any) any {
	switch typed := value.(type) {
	case string:
		return r.String(typed)
	case []string:
		return r.Strings(typed)
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = r.Value(item)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if isSensitiveKey(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = r.Value(item)
			}
		}
		return result
	default:
		return value
	}
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	switch normalized {
	case "token", "password", "passwd", "pwd", "apikey", "api_key", "secret", "authorization", "github_token", "openai_key", "openai_api_key", "aws_access_key", "aws_access_key_id", "aws_secret_access_key", "connection_string", "database_url":
		return true
	default:
		return false
	}
}
