package api

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/auth"
	"github.com/agent-audit-console/agent-audit-console/internal/capture"
	"github.com/agent-audit-console/agent-audit-console/internal/events"
	"github.com/agent-audit-console/agent-audit-console/internal/redact"
	"github.com/go-chi/chi/v5"
)

//go:embed static/* static/assets/*
var staticFiles embed.FS

type Server struct {
	store     *events.SQLiteStore
	readiness func(context.Context) error
	router    chi.Router
}

func New(store *events.SQLiteStore) *Server { return NewWithAuth(store, nil) }

func NewWithAuth(store *events.SQLiteStore, access *auth.Manager, readiness ...func(context.Context) error) *Server {
	check := store.Ready
	if len(readiness) > 0 && readiness[0] != nil {
		check = readiness[0]
	}
	s := &Server{store: store, readiness: check}
	r := chi.NewRouter()
	r.Use(securityHeaders)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, request *http.Request) {
		if err := s.readiness(request.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	r.Route("/api", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(authorize(access, auth.RoleViewer))
			r.Get("/whoami", s.whoami)
			r.Get("/runs", s.runs)
			r.Get("/run-overviews", s.runOverviews)
			r.Get("/runs/{runID}", s.run)
			r.Post("/runs/{runID}/verify", s.verifyRun)
			r.Get("/runs/{runID}/rollback-preview", s.rollbackPreview)
			r.Get("/events/{eventID}", s.event)
		})
		r.With(authorize(access, auth.RoleOperator)).Post("/runs/{runID}/rollback-requests", s.requestRollback)
	})
	content, _ := fs.Sub(staticFiles, "static")
	r.Handle("/*", http.FileServer(http.FS(content)))
	s.router = r
	return s
}

type principalKey struct{}

func authorize(manager *auth.Manager, required auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := auth.Principal{Name: "local", Role: auth.RoleAdmin}
			if manager != nil {
				var ok bool
				principal, ok = manager.Authenticate(r.Header.Get("Authorization"))
				if !ok {
					writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "valid bearer token required"})
					return
				}
			}
			if !auth.Allows(principal.Role, required) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient role"})
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
		})
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.router.ServeHTTP(w, r) }

func (s *Server) whoami(w http.ResponseWriter, r *http.Request) {
	principal, _ := r.Context().Value(principalKey{}).(auth.Principal)
	writeJSON(w, http.StatusOK, principal)
}

func (s *Server) runs(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListRuns(r.Context(), 100)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) runOverviews(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit := int(parseUint(query.Get("limit"), 50))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	filter := events.RunFilter{Agent: query.Get("agent"), Status: query.Get("status"), Risk: query.Get("risk"), Repository: query.Get("repository"), Search: boundedText(query.Get("q"), 256), StartedAfter: query.Get("from"), StartedBefore: query.Get("to"), Cursor: query.Get("cursor"), Limit: limit}
	requestedIntegrity := query.Get("integrity")
	filtered := make([]events.RunOverview, 0, limit+1)
	for len(filtered) <= limit {
		batchLimit := limit
		if requestedIntegrity != "" {
			batchLimit = 100
		}
		filter.Limit = batchLimit
		items, err := s.store.ListRunOverviews(r.Context(), filter)
		if err != nil {
			writeError(w, err)
			return
		}
		available := len(items)
		if available > batchLimit {
			available = batchLimit
		}
		for _, item := range items[:available] {
			item.IntegrityStatus = "verified"
			if verifyErr := s.store.VerifyRun(r.Context(), item.Run.RunID); verifyErr != nil {
				item.IntegrityStatus = "broken"
			}
			item.RollbackStatus, err = s.store.RollbackAvailability(r.Context(), item.Run.RunID)
			if err != nil {
				item.RollbackStatus = "unknown"
			}
			if requestedIntegrity == "" || item.IntegrityStatus == requestedIntegrity {
				filtered = append(filtered, item)
			}
			if len(filtered) > limit {
				break
			}
		}
		if len(filtered) > limit || len(items) <= batchLimit {
			break
		}
		last := items[batchLimit-1].Run
		filter.Cursor = last.StartedAt.UTC().Format(time.RFC3339Nano) + "|" + last.RunID
	}
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}
	cursor := ""
	if hasMore && len(filtered) > 0 {
		last := filtered[len(filtered)-1].Run
		cursor = last.StartedAt.UTC().Format(time.RFC3339Nano) + "|" + last.RunID
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": filtered, "has_more": hasMore, "cursor": cursor})
}

func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "runID")
	run, err := s.store.GetRun(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	after := parseUint(r.URL.Query().Get("after"), 0)
	limit := int(parseUint(r.URL.Query().Get("limit"), 200))
	if limit < 1 || limit > 500 {
		limit = 200
	}
	query := r.URL.Query()
	kinds := parseKinds(query.Get("kind"))
	risks := splitValues(query.Get("risk"))
	statuses := parseActionStatuses(query.Get("status"))
	items, err := s.store.Query(r.Context(), events.Query{RunID: id, After: after, Limit: limit + 1, Kinds: kinds, RiskLevels: risks, ActionStatuses: statuses, Search: boundedText(query.Get("q"), 256)})
	if err != nil {
		writeError(w, err)
		return
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	nextAfter := after
	if len(items) > 0 {
		nextAfter = items[len(items)-1].Sequence
	}
	integrityStatus := "verified"
	if err := s.store.VerifyRun(r.Context(), id); err != nil {
		integrityStatus = "broken"
	}
	rollbackStatus, err := s.store.RollbackAvailability(r.Context(), id)
	if err != nil {
		rollbackStatus = "unknown"
	}
	overviews, _ := s.store.ListRunOverviews(r.Context(), events.RunFilter{RunID: id, Limit: 1})
	var summary any = map[string]any{}
	if len(overviews) > 0 {
		overviews[0].IntegrityStatus = integrityStatus
		overviews[0].RollbackStatus = rollbackStatus
		summary = overviews[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "events": items, "summary": summary, "has_more": hasMore, "next_after": nextAfter, "integrity_status": integrityStatus, "integrity_valid": integrityStatus == "verified", "rollback_status": rollbackStatus})
}

func (s *Server) verifyRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "runID")
	err := s.store.VerifyRun(r.Context(), id)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "verified", "message": "Hash chain verified"})
		return
	}
	if errors.Is(err, events.ErrNotFound) {
		writeError(w, err)
		return
	}
	response := map[string]any{"status": "broken", "message": "Audit evidence may have been modified or corrupted."}
	var integrity *events.IntegrityError
	if errors.As(err, &integrity) {
		response["sequence"] = integrity.Sequence
		response["event_id"] = integrity.EventID
		response["reason"] = integrity.Reason
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) rollbackPreview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "runID")
	run, err := s.store.GetRun(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	items, err := s.store.ListSnapshotsByRun(r.Context(), id, 200)
	if err != nil {
		writeError(w, err)
		return
	}
	files := make([]map[string]any, 0, len(items))
	for _, item := range items {
		currentHash := ""
		currentExists := false
		warnings := []string{}
		if !capture.IsWithin(run.WorkspacePath, item.FilePath) {
			warnings = append(warnings, "Recorded path is outside the workspace")
		} else if _, statErr := os.Lstat(item.FilePath); statErr == nil {
			currentExists = true
			if hash, hashErr := capture.HashFile(item.FilePath); hashErr == nil {
				currentHash = hash
			} else {
				warnings = append(warnings, "Current file could not be hashed")
			}
		} else if !os.IsNotExist(statErr) {
			warnings = append(warnings, "Current file state could not be read")
		}
		conflict := false
		if item.ExpectedExists != nil {
			conflict = currentExists != *item.ExpectedExists || (*item.ExpectedExists && currentHash != item.ExpectedHash)
		} else {
			warnings = append(warnings, "Legacy snapshot has no expected current state")
		}
		files = append(files, map[string]any{"snapshot_id": item.SnapshotID, "action_id": item.ActionID, "path": item.FilePath, "current_hash": currentHash, "target_hash": item.ContentHash, "current_exists": currentExists, "target_exists": item.Exists, "conflict": conflict, "warnings": warnings})
	}
	writeJSON(w, http.StatusOK, map[string]any{"run_id": id, "files": files, "status": map[bool]string{true: "available", false: "unavailable"}[len(files) > 0]})
}

func (s *Server) event(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.ByID(r.Context(), chi.URLParam(r, "eventID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) requestRollback(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")
	if _, err := s.store.GetRun(r.Context(), runID); err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		TargetEventID    string `json:"target_event_id"`
		TargetSnapshotID string `json:"target_snapshot_id"`
		Reason           string `json:"reason"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid rollback request"})
		return
	}
	if input.TargetEventID == "" && input.TargetSnapshotID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target_event_id or target_snapshot_id is required"})
		return
	}
	if input.TargetEventID != "" {
		item, err := s.store.ByID(r.Context(), input.TargetEventID)
		if err != nil || item.RunID != runID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target event does not belong to run"})
			return
		}
	}
	if input.TargetSnapshotID != "" {
		item, err := s.store.SnapshotByID(r.Context(), input.TargetSnapshotID)
		if err != nil || item.RunID != runID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target snapshot does not belong to run"})
			return
		}
	}
	principal, _ := r.Context().Value(principalKey{}).(auth.Principal)
	plan, err := json.Marshal(map[string]string{"reason": redact.New().String(input.Reason)})
	if err != nil {
		writeError(w, err)
		return
	}
	record := events.RollbackRecord{RollbackID: events.NewID("rollback"), RunID: runID, RequestedBy: principal.Name, TargetEventID: input.TargetEventID, TargetSnapshotID: input.TargetSnapshotID, Status: "pending", Plan: plan, RequestedAt: time.Now().UTC()}
	if err := s.store.CreateRollback(r.Context(), record); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"rollback_id": record.RollbackID, "status": record.Status})
}

func parseUint(value string, fallback uint64) uint64 {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func boundedText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
func splitValues(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			result = append(result, item)
		}
	}
	return result
}
func parseKinds(value string) []events.Kind {
	parts := splitValues(value)
	result := make([]events.Kind, len(parts))
	for i, item := range parts {
		result[i] = events.Kind(item)
	}
	return result
}
func parseActionStatuses(value string) []events.ActionStatus {
	parts := splitValues(value)
	result := make([]events.ActionStatus, len(parts))
	for i, item := range parts {
		result[i] = events.ActionStatus(item)
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, events.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "resource not found"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
