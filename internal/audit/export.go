package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/events"
)

type AuditBundle struct {
	SchemaVersion  int            `json:"schema_version"`
	ExportedAt     time.Time      `json:"exported_at"`
	Run            events.Run     `json:"run"`
	Events         []events.Event `json:"events"`
	IntegrityValid bool           `json:"integrity_valid"`
}

func (s *Service) ExportRun(ctx context.Context, runID, format string, writer io.Writer) error {
	bundle, err := s.BuildBundle(ctx, runID)
	if err != nil {
		return err
	}
	switch strings.ToLower(format) {
	case "json":
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(bundle)
	case "html":
		return auditHTML.Execute(writer, bundle)
	default:
		return fmt.Errorf("unsupported export format %q", format)
	}
}

func (s *Service) BuildBundle(ctx context.Context, runID string) (AuditBundle, error) {
	run, err := s.Store.GetRun(ctx, runID)
	if err != nil {
		return AuditBundle{}, err
	}
	var items []events.Event
	var after uint64
	for {
		page, err := s.Store.Query(ctx, events.Query{RunID: runID, After: after, Limit: 1000})
		if err != nil {
			return AuditBundle{}, err
		}
		items = append(items, page...)
		if len(page) < 1000 {
			break
		}
		after = page[len(page)-1].Sequence
	}
	bundle := AuditBundle{SchemaVersion: 1, ExportedAt: time.Now().UTC(), Run: run, Events: items, IntegrityValid: s.Store.VerifyRun(ctx, runID) == nil}
	return bundle, nil
}

func (b AuditBundle) ContentDigest() (string, error) {
	stable := struct {
		SchemaVersion  int            `json:"schema_version"`
		Run            events.Run     `json:"run"`
		Events         []events.Event `json:"events"`
		IntegrityValid bool           `json:"integrity_valid"`
	}{b.SchemaVersion, b.Run, b.Events, b.IntegrityValid}
	encoded, err := json.Marshal(stable)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

var auditHTML = template.Must(template.New("audit").Funcs(template.FuncMap{"json": func(value any) string { data, _ := json.MarshalIndent(value, "", "  "); return string(data) }}).Parse(`<!doctype html><html><head><meta charset="utf-8"><title>Audit {{.Run.RunID}}</title><style>body{font:14px system-ui;max-width:1100px;margin:40px auto;color:#18202a}header{border-bottom:2px solid #18202a}.ok{color:#087f5b}.bad{color:#c92a2a}article{border:1px solid #ccd3da;margin:16px 0;padding:16px;border-radius:8px}pre{white-space:pre-wrap;background:#f4f6f8;padding:12px;overflow:auto}</style></head><body><header><h1>Agent Audit Console</h1><p>Run: {{.Run.RunID}}</p><p>Workspace: {{.Run.WorkspacePath}}</p><p class="{{if .IntegrityValid}}ok{{else}}bad{{end}}">Integrity verified: {{.IntegrityValid}}</p></header>{{range .Events}}<article><h2>#{{.Sequence}} · {{.Kind}}</h2><p>{{.Timestamp}} · {{.Intent}}</p><pre>{{json .}}</pre></article>{{end}}</body></html>`))
