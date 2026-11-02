package runmetrics

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"novelgen/internal/platform/store"
)

func TestFinishWritesExactlyOneRunEndUnderConcurrency(t *testing.T) {
	root := store.MemRoot()
	_, run, err := Start(context.Background(), root, "novelgen write")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run.Finish("", nil)
		}()
	}
	wg.Wait()

	events, err := readEvents(root, run.ID())
	if err != nil {
		t.Fatalf("readEvents() error = %v", err)
	}
	ends := 0
	for _, event := range events {
		if event.Event == EventRunEnd {
			ends++
		}
	}
	if ends != 1 {
		t.Fatalf("run_end events = %d, want 1", ends)
	}
	if err := run.Emit(Event{Event: EventTool}); err == nil {
		t.Fatal("Emit after Finish should fail")
	}
}

func TestListLimitBoundsParsedRuns(t *testing.T) {
	root := store.MemRoot()
	for i := 0; i < 4; i++ {
		_, run, err := Start(context.Background(), root, "novelgen compose")
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		run.Finish("", nil)
	}
	runs, err := List(root, ListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2", len(runs))
	}
}

func TestRetentionPrunesOldestLedgers(t *testing.T) {
	t.Setenv("NOVELGEN_RUN_METRICS_KEEP", "2")
	root := store.MemRoot()
	for i := 0; i < 4; i++ {
		_, run, err := Start(context.Background(), root, "novelgen setup")
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		run.Finish("", nil)
	}
	entries, err := root.FS().ReadDir(root.RunMetricsDir())
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ledger files = %d, want 2", len(entries))
	}
}

func TestReadRejectsMalformedRunID(t *testing.T) {
	root := store.MemRoot()
	for _, id := range []string{"", "..", "../secret", "a/b", strings.Repeat("x", maxRunIDLength+1)} {
		if _, err := Read(root, id); err == nil {
			t.Fatalf("Read(%q) succeeded, want error", id)
		}
	}
}

func TestReadSurfacesNewerSchemaVersion(t *testing.T) {
	root := store.MemRoot()
	_, run, err := Start(context.Background(), root, "novelgen write")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	run.Finish("", nil)
	future, err := json.Marshal(Event{Version: SchemaVersion + 1, RunID: run.ID(), Event: EventAgentCall})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := root.Append(root.RunMetricsFile(run.ID()), append(future, '\n'), store.FileMode); err != nil {
		t.Fatalf("append: %v", err)
	}
	detail, err := Read(root, run.ID())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	found := false
	for _, event := range detail.Events {
		if event.Event == "metric_parse_error" && strings.Contains(event.Error, "unsupported schema version") {
			found = true
		}
	}
	if !found {
		t.Fatalf("future schema version was not surfaced: %#v", detail.Events)
	}
}

func TestMaskCommandRedactsUnknownPayloadFlags(t *testing.T) {
	command := `novelgen tool patch chapter --patch-json={"content":"private"} --title "秘密标题" --id P1-V1-C1`
	masked := MaskCommand(command)
	for _, leak := range []string{"private", "秘密标题"} {
		if strings.Contains(masked, leak) {
			t.Fatalf("MaskCommand leaked %q: %q", leak, masked)
		}
	}
	if !strings.Contains(masked, "--patch-json=<redacted>") {
		t.Fatalf("inline payload not redacted: %q", masked)
	}
	if !strings.Contains(masked, "--id P1-V1-C1") {
		t.Fatalf("structural id should be kept: %q", masked)
	}
}

func TestNormalizeCommandIgnoresEqualsFormIDs(t *testing.T) {
	first := NormalizeCommand("novelgen tool check all --id=C1 --scope chapter")
	second := NormalizeCommand("novelgen tool check all --id=C2 --scope chapter")
	if first != second {
		t.Fatalf("NormalizeCommand() = %q and %q, want equal", first, second)
	}
	if strings.Contains(first, "c1") || strings.Contains(first, "c2") {
		t.Fatalf("NormalizeCommand kept the id value: %q", first)
	}
}
