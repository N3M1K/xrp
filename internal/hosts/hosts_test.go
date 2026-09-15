package hosts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withHosts(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(path, []byte("127.0.0.1 localhost\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HALO_HOSTS_PATH", path)
	return path
}

func TestSyncEntriesDedupesAndRemovesStale(t *testing.T) {
	path := withHosts(t)

	if err := SyncEntries([]string{"a.localhost", "b.localhost", "a.localhost"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Count(content, "a.localhost") != 1 {
		t.Errorf("expected a.localhost exactly once, got:\n%s", content)
	}
	if !strings.Contains(content, "b.localhost") {
		t.Errorf("missing b.localhost:\n%s", content)
	}
	if !strings.Contains(content, "127.0.0.1 localhost") {
		t.Errorf("pre-existing hosts line was lost:\n%s", content)
	}

	// Re-sync with only b: a must disappear.
	if err := SyncEntries([]string{"b.localhost"}); err != nil {
		t.Fatal(err)
	}
	content = string(mustRead(t, path))
	if strings.Contains(content, "a.localhost") {
		t.Errorf("stale entry a.localhost not removed:\n%s", content)
	}
	if strings.Count(content, marker) != 1 {
		t.Errorf("expected exactly 1 managed line, got:\n%s", content)
	}
}

func TestRemoveAllEntries(t *testing.T) {
	path := withHosts(t)
	if err := SyncEntries([]string{"a.localhost"}); err != nil {
		t.Fatal(err)
	}
	if err := RemoveAllEntries(); err != nil {
		t.Fatal(err)
	}
	content := string(mustRead(t, path))
	if strings.Contains(content, marker) {
		t.Errorf("managed entries not removed:\n%s", content)
	}
	if !strings.Contains(content, "localhost") {
		t.Errorf("unrelated content removed:\n%s", content)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
