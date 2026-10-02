package api

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/auth"
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
			r.Get("/runs/{runID}", s.run)
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
	items, err := s.store.Query(r.Context(), events.Query{RunID: id, After: after, Limit: limit + 1})
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
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "events": items, "has_more": hasMore, "next_after": nextAfter, "integrity_status": integrityStatus, "integrity_valid": integrityStatus == "verified", "rollback_status": rollbackStatus})
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
