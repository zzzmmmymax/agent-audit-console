package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTokenConfigStoresOnlyHashAndAuthenticates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.yaml")
	plain, err := AddToken(path, "alice", RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	principal, ok := manager.Authenticate("Bearer " + plain)
	if !ok || principal.Name != "alice" || !Allows(principal.Role, RoleViewer) {
		t.Fatalf("principal=%+v ok=%v", principal, ok)
	}
	if _, ok := manager.Authenticate("Bearer wrong"); ok {
		t.Fatal("wrong token accepted")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), plain) {
		t.Fatal("plaintext token stored in access file")
	}
}
