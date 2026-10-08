package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	authTokenHeader = "X-NovelGen-Token"
	authTokenCookie = "novelgen_token"
)

var (
	// authToken is empty when the server only listens on the loopback
	// interface. Setting it (automatically for non-loopback binds, or via
	// -auth-token) makes every API and WebSocket request require the token.
	authToken string
	// allowedProjectRoots bounds every project path the server will touch.
	allowedProjectRoots []string
	// projectsRootAbs is the absolute books directory used to list and create
	// projects. It replaces the "../books" relative lookups so that the server
	// never depends on the process working directory changing at runtime.
	projectsRootAbs string
)

// newAuthToken returns a random 32 hex character token.
func newAuthToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// isLoopbackHost reports whether host names the local machine.
func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// isLocalOrigin reports whether an Origin header points at the local machine.
// Browser requests carry Origin; command-line clients usually do not.
func isLocalOrigin(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return true
	}
	for _, candidate := range strings.Split(origin, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		// "null" comes from sandboxed iframes and file:// pages; treat it as
		// remote so those documents cannot drive the local API.
		if candidate == "null" {
			return false
		}
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Host == "" {
			return false
		}
		host := parsed.Hostname()
		if !isLoopbackHost(host) {
			return false
		}
	}
	return true
}

func constantTimeEquals(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// tokenMatches accepts the token from a header, the bootstrap cookie, or a
// query parameter (handy for the first browser load and for curl).
func tokenMatches(c *gin.Context) bool {
	if authToken == "" {
		return true
	}
	if constantTimeEquals(c.GetHeader(authTokenHeader), authToken) {
		return true
	}
	if cookie, err := c.Cookie(authTokenCookie); err == nil && constantTimeEquals(cookie, authToken) {
		return true
	}
	if constantTimeEquals(c.Query("token"), authToken) {
		return true
	}
	return false
}

// originGuard rejects browser requests coming from a non-local origin. This is
// what stops a random web page from driving the local API through the user's
// browser, even when the API itself needs no token.
func originGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); origin != "" && !isLocalOrigin(origin) {
			c.AbortWithStatusJSON(http.StatusForbidden, APIResponse{
				Success: false,
				Error:   "cross-origin requests are not allowed",
			})
			return
		}
		c.Next()
	}
}

// authGuard enforces the token on API and WebSocket requests.
func authGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokenMatches(c) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, APIResponse{
			Success: false,
			Error:   "missing or invalid API token; open the URL printed at startup",
		})
	}
}

// bootstrapAuthCookie stores the token in a SameSite=Lax cookie when the page
// is opened with ?token=..., so the single page app can call the API without
// needing any frontend change. Cross-site requests never receive the cookie.
func bootstrapAuthCookie() gin.HandlerFunc {
	return func(c *gin.Context) {
		if authToken != "" && constantTimeEquals(c.Query("token"), authToken) {
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie(authTokenCookie, authToken, 0, "/", "", false, true)
		}
		c.Next()
	}
}

// configureProjectRoots resolves the directories the server may serve. The
// working directory and the books directory are always allowed.
func configureProjectRoots(cwd, configuredRoot string, extra []string) (string, []string, error) {
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return "", nil, err
	}
	if resolved, err := filepath.EvalSymlinks(cwdAbs); err == nil {
		cwdAbs = resolved
	}

	booksDir := strings.TrimSpace(configuredRoot)
	if booksDir == "" {
		sibling := filepath.Join(cwdAbs, "..", "books")
		if info, err := os.Stat(sibling); err == nil && info.IsDir() {
			booksDir = sibling
		} else {
			booksDir = filepath.Join(cwdAbs, "books")
		}
	}
	booksAbs, err := filepath.Abs(booksDir)
	if err != nil {
		return "", nil, err
	}
	if resolved, err := filepath.EvalSymlinks(booksAbs); err == nil {
		booksAbs = resolved
	}

	roots := []string{cwdAbs, booksAbs}
	for _, entry := range extra {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		abs, err := filepath.Abs(entry)
		if err != nil {
			return "", nil, err
		}
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			abs = resolved
		}
		roots = append(roots, abs)
	}

	deduped := make([]string, 0, len(roots))
	for _, root := range roots {
		if !containsString(deduped, root) {
			deduped = append(deduped, root)
		}
	}
	return booksAbs, deduped, nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// withinRoot reports whether target is root itself or lives below it.
func withinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return !filepath.IsAbs(rel)
}

// cleanContentPath normalizes a client supplied relative path and rejects
// traversal, absolute paths, drive letters, and UNC prefixes.
func cleanContentPath(path string) (string, error) {
	raw := strings.TrimSpace(path)
	if raw == "" || strings.ContainsRune(raw, 0) {
		return "", fmt.Errorf("invalid path")
	}
	// Reject UNC shares and absolute paths before normalization can hide them.
	if strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, `\\`) {
		return "", fmt.Errorf("invalid path: %q", path)
	}
	if filepath.IsAbs(raw) && !strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("invalid path: %q", path)
	}
	raw = strings.TrimLeft(strings.ReplaceAll(raw, "\\", "/"), "/")

	parts := make([]string, 0, 8)
	for _, part := range strings.Split(raw, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", fmt.Errorf("invalid path: %q", path)
		}
		if strings.ContainsRune(part, ':') {
			return "", fmt.Errorf("invalid path: %q", path)
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid path")
	}
	clean := filepath.Join(parts...)
	if clean == "." || filepath.IsAbs(clean) {
		return "", fmt.Errorf("invalid path: %q", path)
	}
	return clean, nil
}

// evalExistingAncestor resolves symlinks for the deepest path that exists.
func evalExistingAncestor(path string) string {
	current := path
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return resolved
		}
		parent := filepath.Dir(current)
		if parent == current {
			return current
		}
		current = parent
	}
}

// resolveWithinRoot joins rel onto root and verifies the result stays inside
// root, following symlinks so a link cannot be used to escape.
func resolveWithinRoot(root, rel string) (string, error) {
	clean, err := cleanContentPath(rel)
	if err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = resolved
	}
	full := filepath.Join(rootAbs, clean)
	if !withinRoot(rootAbs, full) {
		return "", fmt.Errorf("path escapes project root: %q", rel)
	}
	if ancestor := evalExistingAncestor(full); !withinRoot(rootAbs, ancestor) {
		return "", fmt.Errorf("path escapes project root: %q", rel)
	}
	return full, nil
}

// resolveProjectDir validates a project path supplied by a client and returns
// its absolute location. Paths must exist and live under an allowed root.
func resolveProjectDir(requested string) (string, error) {
	if strings.TrimSpace(requested) == "" {
		requested = "."
	}
	abs, err := filepath.Abs(requested)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("project path not found: %w", err)
	}
	for _, root := range allowedProjectRoots {
		if withinRoot(root, resolved) {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("project path outside allowed roots: %s", requested)
}

// projectGuard validates the ?project= query parameter before a handler runs,
// and stores the resolved absolute path in the gin context.
func projectGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		requested := strings.TrimSpace(c.Query("project"))
		if requested != "" {
			resolved, err := resolveProjectDir(requested)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusForbidden, APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			c.Set(projectContextKey, resolved)
		}
		c.Next()
	}
}

const projectContextKey = "novelgen_project_root"

// safeSegment rejects path separators and traversal in a single URL segment
// used to build a file name.
func safeSegment(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "." || trimmed == ".." {
		return "", fmt.Errorf("invalid name %q", value)
	}
	if strings.ContainsAny(trimmed, "/\\") || strings.ContainsRune(trimmed, 0) {
		return "", fmt.Errorf("invalid name %q", value)
	}
	return trimmed, nil
}
