# Policy simulation

`audit policy simulate` evaluates a hypothetical action against the active YAML policy. It is a transient, in-memory preflight: it does not execute commands, append audit events, create approvals or rollback records, or modify files.

```shell
audit policy simulate --action git_push --command git,push,origin,main
audit policy simulate --action delete_file --path ./important.txt --json
```

The API exposes the same operation at `POST /api/policy/simulate`; MCP clients use `simulate_policy`. Viewer role is sufficient because these surfaces are read-only. Actual execution always evaluates policy again, so simulation input cannot authorize a later action.

Decisions have one strictness order, defined by the engine: `allow < warn < require_approval < deny`. Risk (`low`, `medium`, `high`) is a separate assessment. All matching rules are returned in stable priority/source/rule-ID order; the strictest decision and highest risk win independently.

Policy documents are limited to 1 MiB, 10,000 rules, 4,096-character strings, and 2,048-character RE2 patterns. API simulation bodies are limited to 64 KiB.
