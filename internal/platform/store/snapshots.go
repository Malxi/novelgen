package store

import (
	"errors"
	"io/fs"
	"path/filepath"
	"time"
)

// timestampLayout is the second-resolution stamp shared by everything that owns
// one file per event: patch checkpoints, backups and log streams. It is part of
// the on-disk contract, because the web UI lists these directories by file name.
// Collisions inside one second are resolved with a numeric suffix rather than by
// raising the resolution (see writeUniqueFile).
const timestampLayout = "20060102_150405"

// SaveCheckpoint snapshots the state that is about to be replaced by a patch, as
// <scope>/checkpoints/<prefix>_<timestamp><ext>, and returns the path.
//
// The patch plane used to spell this out five times, once per section, which is
// why a change to the naming had to be made in five places.
//
// Snapshots accumulate: nothing prunes checkpoints/ or backups/, so a project
// that applies many patches keeps one file per patch. Deleting them is always
// safe (they are never read back by the CLI), but they are also the only record
// of the state before a patch, so the CLI does not delete them on its own.
func (r *Root) SaveCheckpoint(scope Scope, prefix, ext string, data []byte) (string, error) {
	return r.writeSnapshot(r.checkpointsDir(scope), prefix, ext, data)
}

// saveBackup snapshots a file that is about to be regenerated, as
// <scope>/backups/<prefix>_<timestamp><ext>, and returns the path.
func (r *Root) saveBackup(scope Scope, prefix, ext string, data []byte) (string, error) {
	return r.writeSnapshot(r.backupsDir(scope), prefix, ext, data)
}

func (r *Root) writeSnapshot(dir, prefix, ext string, data []byte) (string, error) {
	if prefix == "" {
		return "", errors.New("snapshot prefix is required")
	}
	base := prefix + "_" + time.Now().Format(timestampLayout)
	// Two snapshots of the same artifact can land in the same second (a patch
	// loop, or two patch calls in a row). Overwriting would silently drop the
	// earlier pre-patch state, which is the one thing a snapshot exists to keep,
	// so the name is allocated with O_EXCL and a suffix on collision.
	return writeUniqueFile(r.FS(), dir, base, ext, data, FileMode)
}

// BackupFile copies an existing project file into the scope's backups
// directory, preserving its extension. It returns an empty path when src does
// not exist, so callers can back up "if present" in one step; a source file that
// exists but cannot be read is an error, because silently skipping the backup
// would let the caller overwrite the file thinking it had been preserved.
func (r *Root) BackupFile(scope Scope, src, prefix string) (string, error) {
	data, err := r.Read(src)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		return "", nil
	}
	return r.saveBackup(scope, prefix, filepath.Ext(src), data)
}
