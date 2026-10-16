package store

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"
)

// snapshotLayout is the timestamp format used for checkpoints and backups. It is
// part of the on-disk contract: webui and the CLI both list these directories.
const snapshotLayout = "20060102_150405"

// SaveCheckpoint snapshots the state that is about to be replaced by a patch, as
// <scope>/checkpoints/<prefix>_<timestamp><ext>, and returns the path.
//
// The patch plane used to spell this out five times, once per section, which is
// why a change to the naming had to be made in five places.
func (r *Root) SaveCheckpoint(scope Scope, prefix, ext string, data []byte) (string, error) {
	return r.writeSnapshot(r.CheckpointsDir(scope), prefix, ext, data)
}

// SaveBackup snapshots a file that is about to be regenerated, as
// <scope>/backups/<prefix>_<timestamp><ext>, and returns the path.
func (r *Root) SaveBackup(scope Scope, prefix, ext string, data []byte) (string, error) {
	return r.writeSnapshot(r.BackupsDir(scope), prefix, ext, data)
}

func (r *Root) writeSnapshot(dir, prefix, ext string, data []byte) (string, error) {
	if prefix == "" {
		return "", errors.New("snapshot prefix is required")
	}
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	path := filepath.Join(dir, prefix+"_"+time.Now().Format(snapshotLayout)+ext)
	if err := r.Write(path, data, FileMode); err != nil {
		return "", err
	}
	return path, nil
}

// BackupFile copies an existing project file into the scope's backups
// directory, preserving its extension. It returns an empty path when src does
// not exist, so callers can back up "if present" in one step.
func (r *Root) BackupFile(scope Scope, src, prefix string) (string, error) {
	data, err := r.Read(src)
	if err != nil {
		return "", nil
	}
	return r.SaveBackup(scope, prefix, filepath.Ext(src), data)
}

// WriteJSONSnapshot is SaveCheckpoint for a JSON value.
func (r *Root) WriteJSONSnapshot(scope Scope, prefix string, value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return r.SaveCheckpoint(scope, prefix, ".json", data)
}
