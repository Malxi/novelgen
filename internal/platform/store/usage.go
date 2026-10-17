package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"novelgen/internal/platform/usage"
)

// AppendUsage records one LLM usage entry in logs/usage.jsonl. The ledger path
// lives here with the rest of the layout; usage.Entry keeps only the record
// shape.
func (r *Root) AppendUsage(entry usage.Entry) error {
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal usage entry: %w", err)
	}
	if err := r.Append(r.UsageLog(), append(data, '\n'), FileMode); err != nil {
		return fmt.Errorf("write usage entry: %w", err)
	}
	return nil
}

// ReadUsage returns every recorded usage entry, oldest first. A missing ledger
// yields no entries rather than an error, because a fresh project has not spent
// any tokens yet.
func (r *Root) ReadUsage() ([]usage.Entry, error) {
	data, err := r.Read(r.UsageLog())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	entries := make([]usage.Entry, 0)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var entry usage.Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
