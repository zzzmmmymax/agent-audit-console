package events

import _ "embed"

// SQLiteSchema is the idempotent Stage 0 database schema. A storage adapter
// should execute it once per connection so the connection-scoped pragmas apply.
//
//go:embed schema.sql
var SQLiteSchema string
