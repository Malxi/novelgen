package toolcmd

import (
	"strings"
	"testing"
)

// TestCheckFlagOrder pins the shape every check command is built with: the
// flags appear in one canonical order and optional flags are omitted, which is
// what lets a tool-returned check query and the workflow allowlist that must
// accept it stay identical.
func TestCheckFlagOrder(t *testing.T) {
	cases := []struct {
		name string
		opts CheckOptions
		want string
	}{
		{
			name: "outline chapter with category",
			opts: CheckOptions{Target: "outline", Scope: "chapter", ID: "P1-V1-C1", Category: "logic", MinPriority: "low", MaxIssues: 8},
			want: `novelgen tool check all --target outline --scope chapter --id 'P1-V1-C1' --category logic --min-priority low --max-issues 8`,
		},
		{
			name: "outline volume without id",
			opts: CheckOptions{Target: "outline", Scope: "all", Category: "logic,plot", MinPriority: "low", MaxIssues: 12},
			want: `novelgen tool check all --target outline --scope all --category logic,plot --min-priority low --max-issues 12`,
		},
		{
			name: "setup",
			opts: CheckOptions{Target: "setup", Category: "core_cast", MinPriority: "low", MaxIssues: 12},
			want: `novelgen tool check all --target setup --category core_cast --min-priority low --max-issues 12`,
		},
		{
			name: "craft schema",
			opts: CheckOptions{Kind: "schema", Target: "craft", Scope: "character", ID: "Lin"},
			want: `novelgen tool check schema --target craft --scope character --id 'Lin'`,
		},
		{
			name: "recap quality",
			opts: CheckOptions{Kind: "quality", Target: "recap", Scope: "chapter", ID: "P1-V1-C1", MinPriority: "low", MaxIssues: 8},
			want: `novelgen tool check quality --target recap --scope chapter --id 'P1-V1-C1' --min-priority low --max-issues 8`,
		},
		{
			name: "chapter simulation with target words",
			opts: CheckOptions{Kind: "simulation", Target: "chapter", Scope: "chapter", ID: "P1-V1-C1", Category: "logic,pacing", MinPriority: "low", MaxIssues: 12, TargetWords: 1200},
			want: `novelgen tool check simulation --target chapter --scope chapter --id 'P1-V1-C1' --category logic,pacing --min-priority low --max-issues 12 --target-words 1200`,
		},
	}
	for _, tc := range cases {
		if got := Check(tc.opts); got != tc.want {
			t.Errorf("%s: Check = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestQueryFlagOrder pins the query shape per section, including the flag order
// the runner allowlist matching depends on.
func TestQueryFlagOrder(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "outline chapter detail",
			got:  QueryOutline(QueryOptions{Type: "chapter", ID: "P1-V1-C1", Fields: "storyline_advances,chapter_payoff", View: "brief"}),
			want: `novelgen tool query outline --type chapter --id 'P1-V1-C1' --fields storyline_advances,chapter_payoff --view brief`,
		},
		{
			name: "outline events by chapter",
			got:  QueryOutline(QueryOptions{Type: "events", ChapterID: "P1-V1-C1", Fields: "result,details", View: "brief"}),
			want: `novelgen tool query outline --type events --chapter-id 'P1-V1-C1' --fields result,details --view brief`,
		},
		{
			name: "outline events by volume with limit",
			got:  QueryOutline(QueryOptions{Type: "events", VolumeID: "P1-V1", Fields: "event_index,type", View: "brief", Limit: 12}),
			want: `novelgen tool query outline --type events --volume-id 'P1-V1' --fields event_index,type --view brief --limit 12`,
		},
		{
			name: "outline refs",
			got:  QueryOutline(QueryOptions{Type: "refs", EntityType: "character", Name: "Lin", View: "brief"}),
			want: `novelgen tool query outline --type refs --entity-type character --name 'Lin' --view brief`,
		},
		{
			name: "outline index",
			got:  QueryOutline(QueryOptions{View: "index"}),
			want: `novelgen tool query outline --view index`,
		},
		{
			name: "context outline repair",
			got:  QueryContext(QueryOptions{Type: "outline-repair", ID: "P1-V1-C1", Name: "logic", View: "index"}),
			want: `novelgen tool query context --type outline-repair --id 'P1-V1-C1' --name 'logic' --view index`,
		},
		{
			name: "context craft character",
			got:  QueryContext(QueryOptions{Type: "craft-character", Name: "Lin", View: "brief"}),
			want: `novelgen tool query context --type craft-character --name 'Lin' --view brief`,
		},
		{
			name: "setup search",
			got:  QueryStorySetup(QueryOptions{Type: "search", Name: "insect swarm", View: "brief"}),
			want: `novelgen tool query story-setup --type search --name 'insect swarm' --view brief`,
		},
		{
			name: "chapter content",
			got:  QueryChapter(QueryOptions{ID: "P1-V1-C1", Content: true, View: "brief"}),
			want: `novelgen tool query chapter --id 'P1-V1-C1' --content --view brief`,
		},
		{
			name: "craft name",
			got:  QueryCraft(QueryOptions{Type: "character", Name: "Lin", View: "brief"}),
			want: `novelgen tool query craft --type character --name 'Lin' --view brief`,
		},
		{
			name: "logs index with limit",
			got:  QueryLogs(QueryOptions{View: "index", Limit: 5}),
			want: `novelgen tool query logs --view index --limit 5`,
		},
		{
			name: "logs content by id",
			got:  QueryLogs(QueryOptions{ID: "logs/agent-live/20260101.json", Content: true, View: "brief"}),
			want: `novelgen tool query logs --id 'logs/agent-live/20260101.json' --content --view brief`,
		},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// TestPatchCommandsKeepDryRunDefault pins the patch vocabulary: only an
// explicit Apply adds --apply, so a builder call can never turn a dry run into a
// write by accident.
func TestPatchCommandsKeepDryRunDefault(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"outline volume dry run", PatchOutlineVolume("P1-V1"), `novelgen tool patch outline --target volume --id 'P1-V1'`},
		{"outline volume apply", Patch(PatchOptions{Section: "outline", Target: "volume", ID: "P1-V1", Apply: true}), `novelgen tool patch outline --target volume --id 'P1-V1' --apply`},
		{"outline chapter", Patch(PatchOptions{Section: "outline", Target: "chapter", ID: "P1-V1-C1"}), `novelgen tool patch outline --target chapter --id 'P1-V1-C1'`},
		{"craft", PatchCraft("character", "Lin"), `novelgen tool patch craft --target character --id 'Lin'`},
		{"setup", PatchSetup(), `novelgen tool patch setup`},
		{"setup apply", Patch(PatchOptions{Section: "setup", Apply: true}), `novelgen tool patch setup --apply`},
		{"recap", PatchRecap("P1-V1-C1"), `novelgen tool patch recap --id 'P1-V1-C1'`},
		{"chapter", PatchChapter("P1-V1-C1"), `novelgen tool patch chapter --id 'P1-V1-C1'`},
		{"chapter with buffer", Patch(PatchOptions{Section: "chapter", ID: "P1-V1-C1", Buffer: "P1-V1-C1-draft"}), `novelgen tool patch chapter --id 'P1-V1-C1' --patch-buffer 'P1-V1-C1-draft'`},
		{"chapter apply with refresh", Patch(PatchOptions{Section: "chapter", ID: "P1-V1-C1", Apply: true, RefreshDerived: true}), `novelgen tool patch chapter --id 'P1-V1-C1' --apply --refresh-derived`},
		{
			"chapter apply keeps target words last",
			Patch(PatchOptions{Section: "chapter", ID: "P1-V1-C1", Buffer: "P1-V1-C1-draft", Apply: true, RefreshDerived: true, TargetWords: 800}),
			`novelgen tool patch chapter --id 'P1-V1-C1' --patch-buffer 'P1-V1-C1-draft' --apply --refresh-derived --target-words 800`,
		},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestPatchBufferAndTaskCommands(t *testing.T) {
	if got, want := PatchBufferAppend("P1-V1-C1-draft"), `novelgen tool patch-buffer append --id 'P1-V1-C1-draft' --stdin`; got != want {
		t.Errorf("PatchBufferAppend = %q, want %q", got, want)
	}
	if got, want := PatchTask(PatchOutlineVolume("P1-V1"), "outline-global-repair:structure"),
		`novelgen tool patch outline --target volume --id 'P1-V1' --task 'outline-global-repair:structure'`; got != want {
		t.Errorf("PatchTask = %q, want %q", got, want)
	}
	if got, want := StdInPatchPipe(PatchChapter("P1-V1-C1"), `{"content":"正文"}`),
		`printf '%s' '{"content":"正文"}' | novelgen tool patch chapter --id 'P1-V1-C1'`; got != want {
		t.Errorf("StdInPatchPipe = %q, want %q", got, want)
	}
}

func TestSingleQuoteEscapesEmbeddedQuotes(t *testing.T) {
	if got, want := SingleQuote("a'b"), `'a'"'"'b'`; got != want {
		t.Errorf("SingleQuote = %q, want %q", got, want)
	}
}

func TestShellFacingFlagsUseSingleQuotes(t *testing.T) {
	got := QueryCraft(QueryOptions{Type: "character", Name: `A$(id)B`, View: "brief"})
	if !strings.Contains(got, `--name 'A$(id)B'`) {
		t.Fatalf("query command should single-quote names so shells do not expand substitutions: %s", got)
	}
	patch := PatchCraft("character", `A$(id)B`)
	if !strings.Contains(patch, `--id 'A$(id)B'`) {
		t.Fatalf("patch command should single-quote ids: %s", patch)
	}
}
