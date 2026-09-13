package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestEnsureProjectGeneratesKeyOnce(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	key, created, err := store.EnsureProject(ctx, "default", "Default Project", "")
	if err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if !created {
		t.Fatal("first EnsureProject should report created")
	}
	if key == "" {
		t.Fatal("first EnsureProject should return the generated key")
	}

	// Second call is a no-op: same project, no key re-issued.
	again, created, err := store.EnsureProject(ctx, "default", "Default Project", "")
	if err != nil {
		t.Fatalf("EnsureProject (repeat): %v", err)
	}
	if created || again != "" {
		t.Fatalf("repeat EnsureProject should be a no-op, got created=%v key=%q", created, again)
	}

	ok, err := store.ValidateAPIKey(ctx, "default", key)
	if err != nil || !ok {
		t.Fatalf("original key should still validate (ok=%v err=%v)", ok, err)
	}
}

func TestEnsureProjectAdoptsOperatorKey(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	const operatorKey = "ember_operator_supplied_key"

	returned, created, err := store.EnsureProject(ctx, "default", "Default Project", operatorKey)
	if err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if !created {
		t.Fatal("expected project to be created")
	}
	// The operator already knows this key, so it must not be echoed back.
	if returned != "" {
		t.Fatalf("operator-supplied key should not be returned for printing, got %q", returned)
	}

	ok, err := store.ValidateAPIKey(ctx, "default", operatorKey)
	if err != nil || !ok {
		t.Fatalf("operator key should validate (ok=%v err=%v)", ok, err)
	}
}

func TestSetAPIKeyRotatesWithoutTouchingData(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	oldKey, _, err := store.EnsureProject(ctx, "default", "Default Project", "")
	if err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}

	newKey, err := store.SetAPIKey(ctx, "default", "")
	if err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	if newKey == oldKey {
		t.Fatal("rotation should produce a different key")
	}

	if ok, _ := store.ValidateAPIKey(ctx, "default", oldKey); ok {
		t.Fatal("the old key must stop working after rotation")
	}
	if ok, _ := store.ValidateAPIKey(ctx, "default", newKey); !ok {
		t.Fatal("the new key must work after rotation")
	}

	// The project row survives; rotation is not a re-create.
	projects, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 || projects[0].ID != "default" {
		t.Fatalf("expected the single default project to survive, got %+v", projects)
	}
}

func TestSetAPIKeyUnknownProject(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.SetAPIKey(context.Background(), "nope", ""); err == nil {
		t.Fatal("rotating a key for a missing project should error")
	}
}

func TestValidateAPIKeyRejects(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	key, _, err := store.EnsureProject(ctx, "default", "Default Project", "")
	if err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}

	cases := []struct {
		name      string
		projectID string
		key       string
	}{
		{"wrong key", "default", "ember_not_the_key"},
		{"empty key", "default", ""},
		{"unknown project", "missing", key},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := store.ValidateAPIKey(ctx, tc.projectID, tc.key)
			if err != nil {
				t.Fatalf("ValidateAPIKey: %v", err)
			}
			if ok {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestValidAPIKeyFormat(t *testing.T) {
	valid := []string{
		"ember_0123456789abcdef",
		"a-perfectly-fine-long-key",
	}
	for _, key := range valid {
		if !ValidAPIKeyFormat(key) {
			t.Errorf("ValidAPIKeyFormat(%q) = false, want true", key)
		}
	}

	invalid := []string{
		"",                          // unset variable
		"short",                     // too short to be worth anything
		"  padded-key-with-space  ", // quoting accident in .env
		"trailing-newline-key\n",
	}
	for _, key := range invalid {
		if ValidAPIKeyFormat(key) {
			t.Errorf("ValidAPIKeyFormat(%q) = true, want false", key)
		}
	}
}

func TestGenerateAPIKeyIsUniqueAndPrefixed(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		key, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey: %v", err)
		}
		if len(key) < 16 || key[:6] != "ember_" {
			t.Fatalf("unexpected key shape: %q", key)
		}
		if seen[key] {
			t.Fatalf("duplicate key generated: %q", key)
		}
		seen[key] = true
	}
}
