package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"myapp":             "myapp",
		"My App":            "my-app",
		"my.app":            "my-app",
		"My_App!":           "my-app",
		"  .Hidden Project": "hidden-project",
		"..":                "",
		"foo---bar":         "foo-bar",
		"foo  bar":          "foo-bar",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeDialAddr(t *testing.T) {
	cases := map[string]string{
		"":          "127.0.0.1",
		"*":         "127.0.0.1",
		"0.0.0.0":   "127.0.0.1",
		"::":        "127.0.0.1",
		"localhost": "127.0.0.1",
		"[::1]":     "::1",
		"::1":       "::1",
		"127.0.0.1": "127.0.0.1",
	}
	for in, want := range cases {
		if got := normalizeDialAddr(in); got != want {
			t.Errorf("normalizeDialAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		in       string
		wantHost string
		wantPort string
		wantOK   bool
	}{
		{"0.0.0.0:3000", "0.0.0.0", "3000", true},
		{"127.0.0.1:8096", "127.0.0.1", "8096", true},
		{"[::]:8096", "::", "8096", true},
		{"[::1]:3000", "::1", "3000", true},
		{"*:8080", "*", "8080", true},
		{"nohost", "", "", false},
	}
	for _, tt := range tests {
		host, port, ok := splitHostPort(tt.in)
		if host != tt.wantHost || port != tt.wantPort || ok != tt.wantOK {
			t.Errorf("splitHostPort(%q) = (%q,%q,%v), want (%q,%q,%v)",
				tt.in, host, port, ok, tt.wantHost, tt.wantPort, tt.wantOK)
		}
	}
}

func TestGetProjectName(t *testing.T) {
	dir := t.TempDir()

	// package.json
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"pkg-name"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := getProjectName(dir); got != "pkg-name" {
		t.Errorf("package.json: got %q, want pkg-name", got)
	}

	// Cargo.toml takes over once package.json is gone
	os.Remove(filepath.Join(dir, "package.json"))
	cargo := "[package]\nname = \"cargo-name\"\n\n[dependencies]\nname = \"ignored\"\n"
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargo), 0644); err != nil {
		t.Fatal(err)
	}
	if got := getProjectName(dir); got != "cargo-name" {
		t.Errorf("Cargo.toml: got %q, want cargo-name", got)
	}

	// pyproject.toml fallback
	os.Remove(filepath.Join(dir, "Cargo.toml"))
	py := "[project]\nname = \"py-name\"\n"
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(py), 0644); err != nil {
		t.Fatal(err)
	}
	if got := getProjectName(dir); got != "py-name" {
		t.Errorf("pyproject.toml: got %q, want py-name", got)
	}
}

func TestGetProjectNameFallsBackToDir(t *testing.T) {
	parent := t.TempDir()
	proj := filepath.Join(parent, "my-service")
	if err := os.MkdirAll(proj, 0755); err != nil {
		t.Fatal(err)
	}
	if got := getProjectName(proj); got != "my-service" {
		t.Errorf("got %q, want my-service", got)
	}

	// build/ subdir should resolve to the parent project name
	build := filepath.Join(proj, "build")
	if err := os.MkdirAll(build, 0755); err != nil {
		t.Fatal(err)
	}
	if got := getProjectName(build); got != "my-service" {
		t.Errorf("build dir: got %q, want my-service", got)
	}
}

func TestIsDevContext(t *testing.T) {
	if isDevContext("/usr/bin") {
		t.Error("/usr/bin should not be dev context")
	}
	if isDevContext("/") {
		t.Error("/ should not be dev context")
	}
	if !isDevContext("/home/lukas/dev/myapp") {
		t.Error("home project should be dev context")
	}
}
