// Package store owns the on-disk shape of a novel project.
//
// A novel project used to be addressed two different ways: some commands
// resolved the project root by walking up from the process working directory
// (internal/project), while others assumed the working directory *was* the root
// and built "story/setup/story_setup.json" style paths inline. The same layout
// was therefore re-derived in dozens of files, and tests had to chdir into a
// fixture to exercise any of it.
//
// This package is the single place that knows where project artifacts live.
// Callers hold a *Root, which carries both the project directory and the
// filesystem used to reach it, so nothing here depends on process state.
package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// ErrNoProject is returned when no novel.json can be found by walking up from
// the supplied directory. The wording is part of the CLI's user-visible
// behavior, so it is kept stable.
var ErrNoProject = errors.New("project root not found (novel.json not found in current or parent directories)")

// Root is a handle to one novel project directory.
//
// A Root never consults the process working directory: every path it produces
// is derived from Dir(), and every read and write goes through FS(). That is
// what lets tests hand a command a temporary project, and what will let a
// future backend (a bundle, an archive, a database) replace the filesystem.
type Root struct {
	dir string
	fs  FS
}

// Open returns a Root for dir without requiring the directory to exist yet.
// Callers that need an existing project should use Locate instead.
func Open(dir string) (*Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &Root{dir: filepath.Clean(abs), fs: OSFS{}}, nil
}

// At returns a Root over an already-resolved project directory on the host
// filesystem. It is the constructor for callers that hold a project directory
// string (a Root's Dir(), or a path a command was given) and need the layout
// helpers without another abs/stat round trip. It neither absolutizes nor
// validates the directory: Open resolves, Locate searches.
func At(dir string) *Root {
	return OpenFS(dir, OSFS{})
}

// OpenFS returns a Root over a caller-supplied filesystem. It is the seam used
// by tests (see MemFS) and the entry point for a non-filesystem backend.
//
// The directory is cleaned but deliberately not absolutized, because an
// in-memory filesystem has no notion of the host's absolute paths.
func OpenFS(dir string, fs FS) *Root {
	return &Root{dir: filepath.Clean(dir), fs: fs}
}

// Dir returns the project directory.
func (r *Root) Dir() string {
	if r == nil {
		return ""
	}
	return r.dir
}

// FS returns the filesystem backing this root.
func (r *Root) FS() FS {
	if r == nil {
		return OSFS{}
	}
	return r.fs
}

// String makes a Root usable directly in log and error format strings.
func (r *Root) String() string {
	return r.Dir()
}

// Sub returns the same project scoped to a subdirectory. It exists for callers
// that need to treat a nested directory (for example story/compose) as a root
// of its own, and keeps the filesystem seam intact.
func (r *Root) Sub(name string) *Root {
	return &Root{dir: filepath.Join(r.Dir(), name), fs: r.FS()}
}

// Locate walks up from startDir until it finds a directory containing
// novel.json.
func Locate(startDir string) (*Root, error) {
	return locate(OSFS{}, startDir)
}

// LocateCWD walks up from the process working directory.
func LocateCWD() (*Root, error) {
	return locate(OSFS{}, "")
}

func locate(fs FS, startDir string) (*Root, error) {
	dir := startDir
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dir = cwd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dir = filepath.Clean(abs)
	for {
		candidate := &Root{dir: dir, fs: fs}
		if candidate.Exists(candidate.NovelJSON()) {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, ErrNoProject
		}
		dir = parent
	}
}

// IsProjectDir reports whether dir looks like a novel project directory.
func IsProjectDir(dir string) bool {
	candidate, err := Open(dir)
	if err != nil {
		return false
	}
	return candidate.Exists(candidate.NovelJSON())
}

// defaultRoot carries a process-wide project root for library code that has no
// place to receive one, such as the write pipeline or the recap store. The CLI
// installs it once per invocation (see cmd/root.go); tests may install one
// explicitly instead of chdir-ing. An explicit *Root passed by a caller always
// takes precedence over this fallback.
var defaultRoot struct {
	mu   sync.RWMutex
	root *Root
}

// SetDefault installs the ambient project root used by Default.
func SetDefault(root *Root) {
	defaultRoot.mu.Lock()
	defer defaultRoot.mu.Unlock()
	defaultRoot.root = root
}

// ResetDefault clears the ambient project root.
func ResetDefault() {
	SetDefault(nil)
}

// Default returns the ambient project root: the one installed by SetDefault, or
// the project containing the working directory.
func Default() (*Root, error) {
	defaultRoot.mu.RLock()
	root := defaultRoot.root
	defaultRoot.mu.RUnlock()
	if root != nil {
		return root, nil
	}
	return LocateCWD()
}

type rootContextKey struct{}

// WithRoot attaches an explicit project root to ctx. Commands take their root
// from here, which is what lets a test drive a command against a temporary
// project without touching the process working directory.
func WithRoot(ctx context.Context, root *Root) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, rootContextKey{}, root)
}

// FromContext returns the project root attached to ctx, if any.
func FromContext(ctx context.Context) (*Root, bool) {
	if ctx == nil {
		return nil, false
	}
	root, ok := ctx.Value(rootContextKey{}).(*Root)
	if !ok || root == nil {
		return nil, false
	}
	return root, true
}
