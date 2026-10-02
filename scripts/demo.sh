#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
demo_root=$(mktemp -d "${TMPDIR:-/tmp}/agent-audit-demo.XXXXXX")
workspace="$demo_root/workspace"
data_dir="$demo_root/audit-data"
audit_bin="$demo_root/audit"
mkdir -p "$workspace"

cd "$repo_root"
go build -o "$audit_bin" ./cmd/audit
cd "$workspace"
git init -q
printf 'module example.com/agent-audit-demo\n\ngo 1.26.0\n' > go.mod
printf 'package demo\n\nfunc Value() int { return 1 }\n' > value.go
printf 'package demo\n\nimport "testing"\n\nfunc TestValue(t *testing.T) { if Value() != 2 { t.Fatal(Value()) } }\n' > value_test.go

run_output=$($audit_bin --data-dir "$data_dir" run -C "$workspace" -- sh -c "printf 'package demo\\n\\nfunc Value() int { return 2 }\\n' > value.go; go test ./...")
printf '%s\n' "$run_output"
run_id=$(printf '%s\n' "$run_output" | sed -n 's/^run \([^:]*\):.*/\1/p')
test -n "$run_id"
$audit_bin --data-dir "$data_dir" show "$run_id"
$audit_bin --data-dir "$data_dir" verify "$run_id"
$audit_bin --data-dir "$data_dir" restore "$workspace/value.go"
$audit_bin --data-dir "$data_dir" verify "$run_id"
printf 'Demo completed in %s\n' "$demo_root"
