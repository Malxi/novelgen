package app

import (
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/store"
)

func TestSessionUsesExplicitRootForLazyArtifacts(t *testing.T) {
	root, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	if err := root.SaveConfig(&models.ProjectConfig{Name: "explicit"}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := root.SaveSetup(&models.StorySetup{ProjectName: "explicit", Premise: "premise"}); err != nil {
		t.Fatalf("save setup: %v", err)
	}
	if err := root.SaveOutline(&models.Outline{}); err != nil {
		t.Fatalf("save outline: %v", err)
	}

	other, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open other: %v", err)
	}
	if err := other.SaveConfig(&models.ProjectConfig{Name: "ambient"}); err != nil {
		t.Fatalf("save ambient config: %v", err)
	}
	previous, _ := store.Default()
	store.SetDefault(other)
	t.Cleanup(func() {
		if previous != nil {
			store.SetDefault(previous)
			return
		}
		store.ResetDefault()
	})

	session, err := OpenAt(root)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	if session.Config.Name != "explicit" {
		t.Fatalf("session config = %q, want explicit", session.Config.Name)
	}
	setup, err := session.RequireSetup()
	if err != nil {
		t.Fatalf("require setup: %v", err)
	}
	if setup.ProjectName != "explicit" {
		t.Fatalf("session setup = %q, want explicit", setup.ProjectName)
	}
	if _, err := session.RequireOutline(); err != nil {
		t.Fatalf("require outline: %v", err)
	}
}
