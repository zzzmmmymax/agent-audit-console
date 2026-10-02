package redact

import (
	"strings"
	"testing"
)

func TestRedactsSecretsAndConnectionStrings(t *testing.T) {
	redactor := New()
	input := `token=abc123 --password hunter2 Authorization: Bearer headersecret postgres://user:dbpass@localhost/db {"apikey":"jsonsecret"}`
	result := redactor.String(input)
	for _, secret := range []string{"abc123", "hunter2", "headersecret", "dbpass", "jsonsecret"} {
		if strings.Contains(result, secret) {
			t.Errorf("secret %q remains in %q", secret, result)
		}
	}
	if strings.Count(result, "[REDACTED]") < 5 {
		t.Fatalf("insufficient redaction: %s", result)
	}
}
func TestRedactsStructuredValues(t *testing.T) {
	result := New().Value(map[string]any{"token": "secret", "nested": map[string]any{"password": "hidden"}, "safe": "hello"}).(map[string]any)
	if result["token"] != "[REDACTED]" || result["safe"] != "hello" {
		t.Fatalf("result=%v", result)
	}
}

func TestRedactsProviderKeysAndPrivateKey(t *testing.T) {
	input := "github_pat_abcdefghijklmnopqrstuvwxyz123456 sk-abcdefghijklmnopqrstuvwxyz AKIAABCDEFGHIJKLMNOP -----BEGIN PRIVATE KEY-----\nsecret-material\n-----END PRIVATE KEY-----"
	result := New().String(input)
	for _, secret := range []string{"github_pat_", "sk-abc", "AKIA", "secret-material"} {
		if strings.Contains(result, secret) {
			t.Fatalf("secret marker %q remains in %q", secret, result)
		}
	}
}
