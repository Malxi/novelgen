package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCleanContentPath(t *testing.T) {
	valid := map[string]string{
		"story/compose/outline.json": filepath.Join("story", "compose", "outline.json"),
		"/chapters/chapter-1.md":     filepath.Join("chapters", "chapter-1.md"),
		"logs\\prompts\\a.md":        filepath.Join("logs", "prompts", "a.md"),
	}
	for input, want := range valid {
		got, err := cleanContentPath(input)
		if err != nil {
			t.Fatalf("cleanContentPath(%q) unexpected error: %v", input, err)
		}
		if got != want {
			t.Fatalf("cleanContentPath(%q) = %q, want %q", input, got, want)
		}
	}

	invalid := []string{
		"",
		".",
		"..",
		"../secret.txt",
		"/../secret.txt",
		"a/../../b.txt",
		"..\\..\\secret.txt",
		"C:/Windows/system.ini",
		`\\server\share\file.txt`,
		"story/./../../x",
	}
	for _, input := range invalid {
		if got, err := cleanContentPath(input); err == nil {
			t.Fatalf("cleanContentPath(%q) = %q, want error", input, got)
		}
	}
}

func TestResolveWithinRootRejectsEscape(t *testing.T) {
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}

	if _, err := resolveWithinRoot(root, "story/outline.json"); err != nil {
		t.Fatalf("in-root path should resolve: %v", err)
	}
	for _, escape := range []string{"../outside.txt", "story/../../outside.txt", "/../outside.txt"} {
		if got, err := resolveWithinRoot(root, escape); err == nil {
			t.Fatalf("resolveWithinRoot(%q) = %q, want error", escape, got)
		}
	}
}

func TestResolveWithinRootRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if got, err := resolveWithinRoot(root, "link/secret.txt"); err == nil {
		t.Fatalf("symlink escape resolved to %q, want error", got)
	}
}

// testRoots installs isolated globals so handler tests never touch the real
// project directories.
func testRoots(t *testing.T) (root string, project string) {
	t.Helper()
	root = t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	books := filepath.Join(root, "books")
	if err := os.MkdirAll(books, 0o755); err != nil {
		t.Fatalf("mkdir books: %v", err)
	}
	project = filepath.Join(books, "demo")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "story.txt"), []byte("INSIDE"), 0o644); err != nil {
		t.Fatalf("write project file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("OUTSIDE-SECRET"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	prevRoots, prevBooks, prevToken := allowedProjectRoots, projectsRootAbs, authToken
	allowedProjectRoots = []string{root, books}
	projectsRootAbs = books
	authToken = ""
	t.Cleanup(func() {
		allowedProjectRoots, projectsRootAbs, authToken = prevRoots, prevBooks, prevToken
	})
	return root, project
}

func TestGetFileRejectsTraversalButServesProjectFiles(t *testing.T) {
	_, project := testRoots(t)
	router := buildRouter(false)

	escapeURL := "/api/files/%2e%2e/secret.txt?project=" + url.QueryEscape(project)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, escapeURL, nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("traversal status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "OUTSIDE-SECRET") {
		t.Fatalf("traversal leaked file contents: %s", rec.Body.String())
	}

	okURL := "/api/files/story.txt?project=" + url.QueryEscape(project)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, okURL, nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "INSIDE") {
		t.Fatalf("in-project read failed: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSaveFileRejectsTraversal(t *testing.T) {
	root, project := testRoots(t)
	router := buildRouter(false)

	body := strings.NewReader(`{"content":"pwned"}`)
	escapeURL := "/api/files/%2e%2e/evil.txt?project=" + url.QueryEscape(project)
	req := httptest.NewRequest(http.MethodPost, escapeURL, body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("traversal write status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, "evil.txt")); err == nil {
		t.Fatalf("traversal write created %s", filepath.Join(root, "evil.txt"))
	}
}

func TestProjectGuardRejectsProjectsOutsideAllowedRoots(t *testing.T) {
	testRoots(t)
	outside := t.TempDir()
	router := buildRouter(false)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/content/outline?project="+url.QueryEscape(outside), nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("outside project status = %d, want 403 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestOriginGuardBlocksCrossSiteRequests(t *testing.T) {
	_, project := testRoots(t)
	router := buildRouter(false)

	req := httptest.NewRequest(http.MethodGet, "/api/files/story.txt?project="+url.QueryEscape(project), nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d, want 403", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/files/story.txt?project="+url.QueryEscape(project), nil)
	req.Header.Set("Origin", "http://localhost:8080")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("local origin status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestAuthGuardRequiresTokenWhenConfigured(t *testing.T) {
	_, project := testRoots(t)
	authToken = "test-token"
	router := buildRouter(false)

	target := "/api/files/story.txt?project=" + url.QueryEscape(project)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d, want 401", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set(authTokenHeader, "test-token")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("header token status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, target, nil)
	req.AddCookie(&http.Cookie{Name: authTokenCookie, Value: "test-token"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cookie token status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestIsLocalOrigin(t *testing.T) {
	local := []string{"", "http://localhost:8080", "http://127.0.0.1:5173", "https://[::1]:8443"}
	for _, origin := range local {
		if !isLocalOrigin(origin) {
			t.Fatalf("isLocalOrigin(%q) = false, want true", origin)
		}
	}
	remote := []string{"https://evil.com", "http://192.168.1.10:8080", "null", "not a url"}
	for _, origin := range remote {
		if isLocalOrigin(origin) {
			t.Fatalf("isLocalOrigin(%q) = true, want false", origin)
		}
	}
}

func TestParseAILogFilenameAcceptsUniquenessSuffixes(t *testing.T) {
	cases := map[string]string{
		"ComposeAgent_20260926_120000.md":             "ComposeAgent",
		"ComposeAgent_20260926_120000_123456789.md":   "ComposeAgent",
		"ComposeAgent_20260926_120000_123456789_2.md": "ComposeAgent",
		"WriteAgent_20260926_120000_000000001.json":   "WriteAgent",
	}
	for name, wantAgent := range cases {
		agent, loggedAt := parseAILogFilename(name, time.Time{})
		if agent != wantAgent {
			t.Fatalf("parseAILogFilename(%q) agent = %q, want %q", name, agent, wantAgent)
		}
		if loggedAt.IsZero() {
			t.Fatalf("parseAILogFilename(%q) did not parse a timestamp", name)
		}
	}
}
