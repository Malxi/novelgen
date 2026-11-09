package agentkit

import (
	"strings"
	"testing"
)

func TestBundledGuidanceContent(t *testing.T) {
	workflows, err := ListWorkflows()
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(workflows) != 5 {
		t.Fatalf("workflows = %d, want 5", len(workflows))
	}
	setup, err := ReadWorkflow("setup")
	if err != nil {
		t.Fatalf("ReadWorkflow(setup): %v", err)
	}
	if !strings.Contains(setup, "Setup") && !strings.Contains(setup, "setup") {
		t.Fatalf("setup workflow looks empty:\n%s", setup)
	}

	references, err := ListReferences()
	if err != nil {
		t.Fatalf("ListReferences: %v", err)
	}
	if len(references) < 5 {
		t.Fatalf("references = %d, want at least 5", len(references))
	}
	for _, name := range []string{"setup-schema", "outline-schema"} {
		content, err := ReadReference(name)
		if err != nil {
			t.Fatalf("ReadReference(%s): %v", name, err)
		}
		if !strings.Contains(content, "```json") {
			t.Fatalf("%s reference missing a JSON shape", name)
		}
	}
	setupSchema, err := ReadReference("setup-schema")
	if err != nil {
		t.Fatalf("ReadReference(setup-schema): %v", err)
	}
	if !strings.Contains(setupSchema, "premises") || !strings.Contains(setupSchema, "writing_style") {
		t.Fatalf("setup-schema missing key fields:\n%s", setupSchema)
	}
	outlineSchema, err := ReadReference("outline-schema")
	if err != nil {
		t.Fatalf("ReadReference(outline-schema): %v", err)
	}
	if !strings.Contains(outlineSchema, "\"parts\"") || !strings.Contains(outlineSchema, "\"chapters\"") {
		t.Fatalf("outline-schema missing parts/chapters:\n%s", outlineSchema)
	}
	bookWrite, err := ReadReference("book-write")
	if err != nil {
		t.Fatalf("ReadReference(book-write): %v", err)
	}
	if !strings.Contains(bookWrite, "--mode") {
		t.Fatalf("book-write reference missing mode guidance:\n%s", bookWrite)
	}
	if _, err := ReadReference("../setup"); err == nil {
		t.Fatal("reference traversal should fail")
	}
}
