package events

import "testing"

func TestEventValidate(t *testing.T) {
	event := testEvent(1)
	if err := event.Validate(); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	event.Kind = "unknown"
	if err := event.Validate(); err == nil {
		t.Fatal("unsupported kind accepted")
	}
}

func TestSQLiteSchemaContainsCoreTablesAndWAL(t *testing.T) {
	required := []string{"journal_mode = WAL", "CREATE TABLE IF NOT EXISTS runs", "CREATE TABLE IF NOT EXISTS events", "CREATE TABLE IF NOT EXISTS snapshots", "CREATE TABLE IF NOT EXISTS rollback_records", "previous_hash", "integrity_hash"}
	for _, fragment := range required {
		if !contains(SQLiteSchema, fragment) {
			t.Errorf("schema missing %q", fragment)
		}
	}
}

func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
