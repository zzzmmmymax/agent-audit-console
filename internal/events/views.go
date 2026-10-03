package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type RunFilter struct {
	RunID, Agent, Status, Risk, Repository, Search, StartedAfter, StartedBefore, Cursor string
	Limit                                                                               int
}

type RunOverview struct {
	Run              Run            `json:"run"`
	Repository       string         `json:"repository,omitempty"`
	Branch           string         `json:"branch,omitempty"`
	DurationMS       int64          `json:"duration_ms"`
	ActionCount      int            `json:"action_count"`
	EventCount       int            `json:"event_count"`
	FilesChanged     int            `json:"files_changed"`
	Commands         int            `json:"commands"`
	Tests            int            `json:"tests"`
	ApprovalCount    int            `json:"approval_count"`
	PendingApprovals int            `json:"pending_approvals"`
	RiskCounts       map[string]int `json:"risk_counts"`
	IntegrityStatus  string         `json:"integrity_status"`
	RollbackStatus   string         `json:"rollback_status"`
}

func (s *SQLiteStore) ListRunOverviews(ctx context.Context, filter RunFilter) ([]RunOverview, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}
	statement := `SELECT r.run_id,r.agent_type,r.agent_id,r.status,r.workspace_path,r.started_at,r.ended_at,r.head_hash,r.metadata_json,
		COUNT(e.event_id),COUNT(DISTINCT e.action_id),
		SUM(CASE WHEN e.kind='file_change' THEN 1 ELSE 0 END),SUM(CASE WHEN e.kind='command' THEN 1 ELSE 0 END),
		SUM(CASE WHEN e.kind='command' AND (json_extract(e.evidence_json,'$.data.test')=1 OR json_extract(e.evidence_json,'$.data.category')='test') THEN 1 ELSE 0 END),
		SUM(CASE WHEN e.kind='approval' THEN 1 ELSE 0 END),SUM(CASE WHEN e.kind='approval' AND json_extract(e.policy_decision_json,'$.status')='pending' THEN 1 ELSE 0 END),
		SUM(CASE WHEN json_extract(e.risk_json,'$.level')='low' THEN 1 ELSE 0 END),SUM(CASE WHEN json_extract(e.risk_json,'$.level')='medium' THEN 1 ELSE 0 END),SUM(CASE WHEN json_extract(e.risk_json,'$.level')='high' THEN 1 ELSE 0 END)
		FROM runs r LEFT JOIN events e ON e.run_id=r.run_id WHERE 1=1`
	args := []any{}
	add := func(clause string, value string) {
		if strings.TrimSpace(value) != "" {
			statement += clause
			args = append(args, value)
		}
	}
	add(` AND r.agent_type=?`, filter.Agent)
	add(` AND r.status=?`, filter.Status)
	add(` AND json_extract(r.metadata_json,'$.repository')=?`, filter.Repository)
	add(` AND r.started_at>=?`, filter.StartedAfter)
	add(` AND r.started_at<=?`, filter.StartedBefore)
	add(` AND r.run_id=?`, filter.RunID)
	if filter.Risk != "" {
		statement += ` AND EXISTS(SELECT 1 FROM events er WHERE er.run_id=r.run_id AND json_extract(er.risk_json,'$.level')=?)`
		args = append(args, filter.Risk)
	}
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		statement += ` AND (r.run_id LIKE ? OR r.agent_type LIKE ? OR r.agent_id LIKE ? OR json_extract(r.metadata_json,'$.repository') LIKE ? OR json_extract(r.metadata_json,'$.git_branch') LIKE ?)`
		args = append(args, like, like, like, like, like)
	}
	if filter.Cursor != "" {
		parts := strings.SplitN(filter.Cursor, "|", 2)
		if len(parts) == 2 {
			statement += ` AND (r.started_at<? OR (r.started_at=? AND r.run_id<?))`
			args = append(args, parts[0], parts[0], parts[1])
		}
	}
	statement += ` GROUP BY r.run_id ORDER BY r.started_at DESC,r.run_id DESC LIMIT ?`
	args = append(args, filter.Limit+1)
	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []RunOverview{}
	for rows.Next() {
		var item RunOverview
		var started, metadata string
		var ended sql.NullString
		var eventsCount, actions, files, commands, tests, approvals, pending, low, medium, high sql.NullInt64
		if err := rows.Scan(&item.Run.RunID, &item.Run.AgentType, &item.Run.AgentID, &item.Run.Status, &item.Run.WorkspacePath, &started, &ended, &item.Run.HeadHash, &metadata, &eventsCount, &actions, &files, &commands, &tests, &approvals, &pending, &low, &medium, &high); err != nil {
			return nil, err
		}
		item.Run.StartedAt, err = parseTime(started)
		if err != nil {
			return nil, err
		}
		if ended.Valid {
			value, e := parseTime(ended.String)
			if e != nil {
				return nil, e
			}
			item.Run.EndedAt = &value
		}
		if err = json.Unmarshal([]byte(metadata), &item.Run.Metadata); err != nil {
			return nil, err
		}
		item.Repository = item.Run.Metadata["repository"]
		item.Branch = item.Run.Metadata["git_branch"]
		item.EventCount = int(eventsCount.Int64)
		item.ActionCount = int(actions.Int64)
		item.FilesChanged = int(files.Int64)
		item.Commands = int(commands.Int64)
		item.Tests = int(tests.Int64)
		item.ApprovalCount = int(approvals.Int64)
		item.PendingApprovals = int(pending.Int64)
		item.RiskCounts = map[string]int{RiskLow: int(low.Int64), RiskMedium: int(medium.Int64), RiskHigh: int(high.Int64)}
		end := time.Now().UTC()
		if item.Run.EndedAt != nil {
			end = *item.Run.EndedAt
		}
		item.DurationMS = end.Sub(item.Run.StartedAt).Milliseconds()
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) ListSnapshotsByRun(ctx context.Context, runID string, limit int) ([]SnapshotRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, snapshotSelect+` WHERE run_id=? ORDER BY captured_at DESC LIMIT ?`, runID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []SnapshotRecord{}
	for rows.Next() {
		item, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
