// Package storagecmd exposes the read/write project-storage surface used by the
// web UI and by scripts that need the same layout/backend semantics as the CLI.
package storagecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"

	"novelgen/internal/platform/store"

	"github.com/spf13/cobra"
)

type entry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path,omitempty"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size,omitempty"`
	ModTime time.Time `json:"mod_time,omitempty"`
}

type project struct {
	Key    string          `json:"key"`
	Path   string          `json:"path"`
	Name   string          `json:"name,omitempty"`
	Exists bool            `json:"exists"`
	Config json.RawMessage `json:"config,omitempty"`
}

type response struct {
	OK       bool      `json:"ok"`
	Code     string    `json:"code,omitempty"`
	Error    string    `json:"error,omitempty"`
	Content  string    `json:"content,omitempty"`
	Size     int64     `json:"size,omitempty"`
	Entry    *entry    `json:"entry,omitempty"`
	Entries  []entry   `json:"entries,omitempty"`
	Projects []project `json:"projects,omitempty"`
}

var (
	storageProject string
	storagePath    string
	storageRoot    string
)

// NewCommand returns the `novelgen storage` command tree.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "storage",
		Short:         "Read and write project files through the configured storage backend",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().StringVar(&storageProject, "project", "", "Project directory or backend project key")
	cmd.PersistentFlags().StringVar(&storagePath, "path", ".", "Project-relative file or directory path")

	projectsCmd := &cobra.Command{
		Use:   "projects",
		Short: "List projects directly below a workspace root",
		RunE:  runProjects,
	}
	projectsCmd.Flags().StringVar(&storageRoot, "root", ".", "Workspace directory that contains project folders")

	readCmd := &cobra.Command{
		Use:   "read",
		Short: "Read one project-relative file as JSON",
		RunE:  runRead,
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List one project-relative directory",
		RunE:  runList,
	}
	statCmd := &cobra.Command{
		Use:   "stat",
		Short: "Stat one project-relative path",
		RunE:  runStat,
	}
	writeCmd := &cobra.Command{
		Use:   "write",
		Short: "Write stdin to one project-relative file",
		RunE:  runWrite,
	}
	replaceCmd := &cobra.Command{
		Use:   "replace",
		Short: "Replace one canonical artifact through the project mutation service",
		Long: `Replace one complete canonical artifact (setup, outline, craft, chapter or recap).

Unlike "storage write", this command parses and validates the artifact, writes a
checkpoint, normalizes it and refreshes derived files through the same mutation
service that "tool patch" uses.`,
		RunE: runReplace,
	}
	for _, sub := range []*cobra.Command{projectsCmd, readCmd, listCmd, statCmd, writeCmd, replaceCmd} {
		sub.SilenceUsage = true
		sub.SilenceErrors = true
	}
	cmd.AddCommand(projectsCmd, readCmd, listCmd, statCmd, writeCmd, replaceCmd)
	return cmd
}

func runProjects(cmd *cobra.Command, _ []string) error {
	infos, err := store.ListProjects(storageRoot)
	if err != nil {
		return fail(cmd, "storage_error", err)
	}
	projects := make([]project, 0, len(infos))
	for _, info := range infos {
		projects = append(projects, project{
			Key:    info.Key,
			Path:   info.Path,
			Name:   info.Name,
			Exists: info.Exists,
			Config: append(json.RawMessage(nil), info.Config...),
		})
	}
	return succeed(cmd, response{OK: true, Projects: projects})
}

func runRead(cmd *cobra.Command, _ []string) error {
	root, err := openProject()
	if err != nil {
		return fail(cmd, "invalid_project", err)
	}
	data, err := root.ReadRelative(storagePath)
	if err != nil {
		return fail(cmd, storageCode(err), err)
	}
	return succeed(cmd, response{OK: true, Content: string(data), Size: int64(len(data))})
}

func runList(cmd *cobra.Command, _ []string) error {
	root, err := openProject()
	if err != nil {
		return fail(cmd, "invalid_project", err)
	}
	dirEntries, err := root.ListRelative(storagePath)
	if err != nil {
		return fail(cmd, storageCode(err), err)
	}
	entries := make([]entry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		item := entry{Name: dirEntry.Name(), IsDir: dirEntry.IsDir()}
		if !item.IsDir {
			if info, infoErr := dirEntry.Info(); infoErr == nil {
				item.Size = info.Size()
				item.ModTime = info.ModTime()
			}
		}
		entries = append(entries, item)
	}
	return succeed(cmd, response{OK: true, Entries: entries})
}

func runStat(cmd *cobra.Command, _ []string) error {
	root, err := openProject()
	if err != nil {
		return fail(cmd, "invalid_project", err)
	}
	info, err := root.StatRelative(storagePath)
	if err != nil {
		return fail(cmd, storageCode(err), err)
	}
	item := entry{
		Name:    info.Name(),
		Path:    storagePath,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
	return succeed(cmd, response{OK: true, Entry: &item})
}

func runWrite(cmd *cobra.Command, _ []string) error {
	root, err := openProject()
	if err != nil {
		return fail(cmd, "invalid_project", err)
	}
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return fail(cmd, "read_stdin", err)
	}
	if err := root.WriteRelative(storagePath, data, store.FileMode); err != nil {
		return fail(cmd, storageCode(err), err)
	}
	return succeed(cmd, response{OK: true})
}

func openProject() (*store.Root, error) {
	if storageProject == "" {
		return nil, fmt.Errorf("--project is required")
	}
	return store.OpenProject(storageProject)
}

func succeed(cmd *cobra.Command, value response) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}

func fail(cmd *cobra.Command, code string, err error) error {
	if err == nil {
		err = errors.New("storage operation failed")
	}
	data, marshalErr := json.Marshal(response{OK: false, Code: code, Error: err.Error()})
	if marshalErr == nil {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	}
	return err
}

func storageCode(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "not_found"
	case errors.Is(err, fs.ErrPermission):
		return "unreadable"
	case errors.Is(err, fs.ErrInvalid):
		return "invalid_path"
	default:
		return "storage_error"
	}
}
