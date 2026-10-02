package syncclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/audit"
	"github.com/agent-audit-console/agent-audit-console/internal/events"
)

func TestPushUsesAuthAndIdempotency(t *testing.T) {
	var received audit.AuditBundle
	var idempotency string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Idempotency-Key") == "" {
			t.Error("missing headers")
		}
		idempotency = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"remote_id":"remote-1","status":"stored"}`))
	}))
	defer server.Close()
	bundle := audit.AuditBundle{SchemaVersion: 1, IntegrityValid: true, Run: events.Run{RunID: "run-1", HeadHash: "abc"}}
	receipt, err := (&Client{Endpoint: server.URL, Token: "secret", AllowHTTP: true}).Push(context.Background(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.RemoteID != "remote-1" || received.Run.RunID != "run-1" {
		t.Fatalf("receipt=%+v bundle=%+v", receipt, received)
	}
	firstKey := idempotency
	bundle.ExportedAt = time.Now().Add(time.Hour)
	if _, err := (&Client{Endpoint: server.URL, Token: "secret", AllowHTTP: true}).Push(context.Background(), bundle); err != nil {
		t.Fatal(err)
	}
	if idempotency != firstKey {
		t.Fatalf("idempotency key changed with export time: %s != %s", idempotency, firstKey)
	}
}
func TestPushRejectsInvalidChainAndPlainHTTP(t *testing.T) {
	client := &Client{Endpoint: "http://example.com", Token: "secret"}
	if _, err := client.Push(context.Background(), audit.AuditBundle{IntegrityValid: false}); err == nil {
		t.Fatal("invalid chain accepted")
	}
	if _, err := client.Push(context.Background(), audit.AuditBundle{IntegrityValid: true}); err == nil {
		t.Fatal("plain HTTP accepted")
	}
}

func TestPushRequiresToken(t *testing.T) {
	client := &Client{Endpoint: "https://example.com"}
	if _, err := client.Push(context.Background(), audit.AuditBundle{IntegrityValid: true}); err == nil {
		t.Fatal("missing token accepted")
	}
}

func TestPushRejectsCredentialsOrQueryInEndpoint(t *testing.T) {
	bundle := audit.AuditBundle{IntegrityValid: true, Run: events.Run{RunID: "run-1"}}
	for _, endpoint := range []string{"https://user:pass@example.com", "https://example.com?token=secret"} {
		if _, err := (&Client{Endpoint: endpoint, Token: "secret"}).Push(context.Background(), bundle); err == nil {
			t.Fatalf("unsafe endpoint accepted: %s", endpoint)
		}
	}
}

func TestRemoteErrorIsRedacted(t *testing.T) {
	secret := "ghp_abcdefghijklmnopqrstuvwxyz123456"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "token="+secret, http.StatusBadRequest)
	}))
	defer server.Close()
	bundle := audit.AuditBundle{IntegrityValid: true, Run: events.Run{RunID: "run-1"}}
	_, err := (&Client{Endpoint: server.URL, Token: "secret", AllowHTTP: true}).Push(context.Background(), bundle)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("error=%v", err)
	}
}
