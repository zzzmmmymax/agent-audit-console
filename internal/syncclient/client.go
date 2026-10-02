// Package syncclient implements explicit opt-in remote audit bundle delivery.
package syncclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/audit"
	"github.com/agent-audit-console/agent-audit-console/internal/redact"
)

type Client struct {
	Endpoint, Token string
	HTTPClient      *http.Client
	AllowHTTP       bool
}
type Receipt struct {
	RemoteID string `json:"remote_id,omitempty"`
	Status   string `json:"status"`
}

func (c *Client) Push(ctx context.Context, bundle audit.AuditBundle) (Receipt, error) {
	if strings.TrimSpace(c.Token) == "" {
		return Receipt{}, errors.New("sync token is required")
	}
	if !bundle.IntegrityValid {
		return Receipt{}, errors.New("refusing to sync an invalid event chain")
	}
	endpoint, err := url.Parse(c.Endpoint)
	if err != nil {
		return Receipt{}, err
	}
	if endpoint.Scheme != "https" && !(c.AllowHTTP && endpoint.Scheme == "http") {
		return Receipt{}, errors.New("remote sync requires HTTPS")
	}
	if endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return Receipt{}, errors.New("sync endpoint must be an absolute base URL without credentials, query, or fragment")
	}
	digest, err := bundle.ContentDigest()
	if err != nil {
		return Receipt{}, err
	}
	payload, err := json.Marshal(bundle)
	if err != nil {
		return Receipt{}, err
	}
	target := strings.TrimRight(c.Endpoint, "/") + "/v1/audit/runs/" + url.PathEscape(bundle.Run.RunID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return Receipt{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.Token)
	request.Header.Set("Idempotency-Key", bundle.Run.RunID+":"+digest)
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return Receipt{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
		if readErr != nil {
			return Receipt{}, fmt.Errorf("remote sync failed: %s", response.Status)
		}
		return Receipt{}, fmt.Errorf("remote sync failed: %s: %s", response.Status, redact.New().String(strings.TrimSpace(string(message))))
	}
	receipt := Receipt{Status: "accepted"}
	_ = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&receipt)
	if receipt.Status == "" {
		receipt.Status = "accepted"
	}
	return receipt, nil
}
