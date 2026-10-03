#!/usr/bin/env sh
set -eu
data_dir="${1:-.tmp/web-audit-demo}"
large_events="${2:-1000}"
go run ./cmd/audit-demo-data --data-dir "$data_dir" --large-events "$large_events"
printf 'Start the console with: go run ./cmd/auditd --data-dir %s\n' "$data_dir"
