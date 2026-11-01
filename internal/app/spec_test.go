package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sampleSpec() SpecContent {
	return SpecContent{
		Title:  "Specs as entities",
		Status: "draft",
		Stories: []SpecStory{{
			ID: "US-1", Title: "Read a spec", Priority: "P1", Description: "Open the page.", Why: "Core value.", IndependentTest: "Open it.",
			Scenarios: []SpecScenario{{Given: "a spec", When: "I open it", Then: "I see it"}},
		}},
		EdgeCases:       []string{"What if empty?"},
		Requirements:    []SpecRequirement{{ID: "FR-001", Text: "System MUST store specs."}, {ID: "FR-002", Text: "System MUST keep revisions.", Clarify: "How many?"}},
		Entities:        []SpecEntity{{Name: "Spec", Description: "A document."}},
		SuccessCriteria: []SpecCriterion{{ID: "SC-001", Text: "Found in 5 seconds."}},
		Clarifications:  []SpecClarificationSession{{Date: "2026-11-01", Items: []SpecClarification{{Question: "Where?", Answer: "InfTask."}}}},
	}
}

func specFixture(t *testing.T) *Service {
	t.Helper()
	s := inWorkspace(t, mustOpen(t), "default")
	alTeam(t, s)
	return s
}

func mustOpen(t *testing.T) *Service {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "spec.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSpecLifecycle(t *testing.T) {
	s := specFixture(t)
	ctx := context.Background()
	first := sampleSpec()
	created, err := s.CreateSpec(ctx, CreateSpec{Team: "AL", Content: first, Message: "Initial draft"})
	if err != nil || created.Identifier != "SPEC-1" || created.Team != "AL" || created.Revision != 1 || !reflect.DeepEqual(created.Content, first) {
		t.Fatal(created, err)
	}
	second := sampleSpec()
	second.Status = "review"
	second.Requirements = append(second.Requirements, SpecRequirement{ID: "FR-003", Text: "System MUST list specs."})
	updated, err := s.UpdateSpec(ctx, UpdateSpec{Identifier: "SPEC-1", BaseRevision: 1, Content: second, Message: "Add listing"})
	if err != nil || updated.Revision != 2 || !reflect.DeepEqual(updated.Content, second) {
		t.Fatal(updated, err)
	}
	got, err := s.GetSpec(ctx, "SPEC-1")
	if err != nil || !reflect.DeepEqual(got, updated) {
		t.Fatal(got, err)
	}
	revisions, err := s.ListSpecRevisions(ctx, "SPEC-1")
	if err != nil || len(revisions) != 2 || revisions[0].Number != 2 || revisions[1].Number != 1 || revisions[0].Message != "Add listing" {
		t.Fatal(revisions, err)
	}
	old, err := s.GetSpecRevision(ctx, GetSpecRevision{Identifier: "SPEC-1", Number: 1})
	if err != nil || old.Number != 1 || !reflect.DeepEqual(old.Content, first) {
		t.Fatal(old, err)
	}
	list, err := s.ListSpecs(ctx, "")
	if err != nil || len(list) != 1 || list[0].Identifier != "SPEC-1" || list[0].Title != first.Title || list[0].Status != "review" || list[0].Revision != 2 {
		t.Fatal(list, err)
	}
	other, err := s.CreateSpec(ctx, CreateSpec{Team: "AL", Content: sampleSpec()})
	if err != nil || other.Identifier != "SPEC-2" {
		t.Fatal(other, err)
	}
	if _, err = s.GetSpec(ctx, "SPEC-9"); !errors.Is(err, ErrSpecNotFound) {
		t.Fatal(err)
	}
}

func TestSpecRejectsStaleBaseRevision(t *testing.T) {
	s := specFixture(t)
	ctx := context.Background()
	if _, err := s.CreateSpec(ctx, CreateSpec{Team: "AL", Content: sampleSpec()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateSpec(ctx, UpdateSpec{Identifier: "SPEC-1", BaseRevision: 1, Content: sampleSpec()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateSpec(ctx, UpdateSpec{Identifier: "SPEC-1", BaseRevision: 1, Content: sampleSpec()}); !errors.Is(err, ErrSpecConflict) || KindOf(err) != KindConflict {
		t.Fatal(err)
	}
	revisions, err := s.ListSpecRevisions(ctx, "SPEC-1")
	if err != nil || len(revisions) != 2 {
		t.Fatal(revisions, err)
	}
}

func TestSpecRevisionsAreAppendOnly(t *testing.T) {
	s := specFixture(t)
	if _, err := s.CreateSpec(context.Background(), CreateSpec{Team: "AL", Content: sampleSpec()}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{`UPDATE spec_revisions SET message='changed'`, `UPDATE spec_revisions SET content='{}'`, `DELETE FROM spec_revisions`} {
		if _, err := s.db.Exec(statement); err == nil {
			t.Fatal("accepted", statement)
		}
	}
}

func TestSpecValidation(t *testing.T) {
	s := specFixture(t)
	ctx := context.Background()
	for name, mutate := range map[string]func(*SpecContent){
		"empty title":         func(c *SpecContent) { c.Title = " " },
		"unknown status":      func(c *SpecContent) { c.Status = "done" },
		"unknown priority":    func(c *SpecContent) { c.Stories[0].Priority = "P4" },
		"story without title": func(c *SpecContent) { c.Stories[0].Title = "" },
		"incomplete scenario": func(c *SpecContent) { c.Stories[0].Scenarios[0].Then = "" },
		"bad story id":        func(c *SpecContent) { c.Stories[0].ID = "STORY-1" },
		"bad requirement id":  func(c *SpecContent) { c.Requirements[0].ID = "REQ-1" },
		"bad criterion id":    func(c *SpecContent) { c.SuccessCriteria[0].ID = "FR-001" },
		"duplicate FR":        func(c *SpecContent) { c.Requirements[1].ID = "FR-001" },
		"empty requirement":   func(c *SpecContent) { c.Requirements[0].Text = "" },
		"bad clarification":   func(c *SpecContent) { c.Clarifications[0].Date = "yesterday" },
		"empty answer":        func(c *SpecContent) { c.Clarifications[0].Items[0].Answer = "" },
		"entity without name": func(c *SpecContent) { c.Entities[0].Name = "" },
		"oversized":           func(c *SpecContent) { c.EdgeCases = []string{strings.Repeat("x", 300*1024)} },
	} {
		t.Run(name, func(t *testing.T) {
			content := sampleSpec()
			mutate(&content)
			if _, err := s.CreateSpec(ctx, CreateSpec{Team: "AL", Content: content}); !errors.Is(err, ErrInvalidSpec) {
				t.Fatal(err)
			}
		})
	}
	if _, err := s.CreateSpec(ctx, CreateSpec{Team: "AL", Content: sampleSpec(), Message: "two\nlines"}); !errors.Is(err, ErrInvalidSpec) {
		t.Fatal(err)
	}
	if _, err := s.CreateSpec(ctx, CreateSpec{Team: "NOPE", Content: sampleSpec()}); !errors.Is(err, ErrTeamNotFound) {
		t.Fatal(err)
	}
	if list, err := s.ListSpecs(ctx, ""); err != nil || len(list) != 0 {
		t.Fatal("invalid input created a spec", list, err)
	}
}

func TestSpecIDsAreNotReused(t *testing.T) {
	s := specFixture(t)
	ctx := context.Background()
	if _, err := s.CreateSpec(ctx, CreateSpec{Team: "AL", Content: sampleSpec()}); err != nil {
		t.Fatal(err)
	}
	removed := sampleSpec()
	removed.Requirements = removed.Requirements[:1]
	if _, err := s.UpdateSpec(ctx, UpdateSpec{Identifier: "SPEC-1", BaseRevision: 1, Content: removed}); err != nil {
		t.Fatal(err)
	}
	reused := sampleSpec()
	reused.Requirements = []SpecRequirement{{ID: "FR-001", Text: "Same."}, {ID: "FR-002", Text: "A different requirement under an old ID."}}
	if _, err := s.UpdateSpec(ctx, UpdateSpec{Identifier: "SPEC-1", BaseRevision: 2, Content: reused}); !errors.Is(err, ErrInvalidSpec) {
		t.Fatal(err)
	}
	fresh := sampleSpec()
	fresh.Requirements = []SpecRequirement{{ID: "FR-001", Text: "Same."}, {ID: "FR-003", Text: "A new requirement."}}
	if _, err := s.UpdateSpec(ctx, UpdateSpec{Identifier: "SPEC-1", BaseRevision: 2, Content: fresh}); err != nil {
		t.Fatal(err)
	}
}

func TestSpecPrivateTeamVisibility(t *testing.T) {
	_, s, admin, member := accessFixture(t)
	if _, err := s.CreateTeam(member, CreateTeam{Key: "SEC", Name: "Sensitive", Private: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSpec(admin, CreateSpec{Team: "SEC", Content: sampleSpec()}); !errors.Is(err, ErrTeamNotFound) {
		t.Fatal("admin created in a private team before joining", err)
	}
	created, err := s.CreateSpec(member, CreateSpec{Team: "SEC", Content: sampleSpec()})
	if err != nil || created.Author == nil || created.Author.Subject != "member" {
		t.Fatal(created, err)
	}
	if _, err = s.GetSpec(admin, created.Identifier); !errors.Is(err, ErrSpecNotFound) {
		t.Fatal(err)
	}
	if _, err = s.UpdateSpec(admin, UpdateSpec{Identifier: created.Identifier, BaseRevision: 1, Content: sampleSpec()}); !errors.Is(err, ErrSpecNotFound) {
		t.Fatal(err)
	}
	if _, err = s.ListSpecRevisions(admin, created.Identifier); !errors.Is(err, ErrSpecNotFound) {
		t.Fatal(err)
	}
	if _, err = s.GetSpecRevision(admin, GetSpecRevision{Identifier: created.Identifier, Number: 1}); !errors.Is(err, ErrSpecNotFound) {
		t.Fatal(err)
	}
	if list, err := s.ListSpecs(admin, ""); err != nil || len(list) != 0 {
		t.Fatal(list, err)
	}
	if list, err := s.ListSpecs(member, ""); err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
}

func TestSpecTeamMustShareWorkspace(t *testing.T) {
	root := mustOpen(t)
	a := inWorkspace(t, root, "alpha")
	b := inWorkspace(t, root, "beta")
	if _, err := a.CreateTeam(context.Background(), CreateTeam{Key: "AA", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.CreateTeam(context.Background(), CreateTeam{Key: "BB", Name: "B"}); err != nil {
		t.Fatal(err)
	}
	_, err := root.db.Exec(`INSERT INTO specs(workspace_id,team_id,number,current_revision,created_at,updated_at)
 VALUES((SELECT id FROM workspaces WHERE slug='alpha'),(SELECT id FROM teams WHERE key='BB'),1,1,'t','t')`)
	if err == nil {
		t.Fatal("accepted a team from another workspace")
	}
}

func removeSpecSchema(t *testing.T, s *Service) {
	t.Helper()
	if _, err := s.db.Exec(`DROP TABLE spec_revisions; DROP TABLE specs;`); err != nil {
		t.Fatal(err)
	}
}

func TestSpecUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := inWorkspace(t, root, "default")
	alTeam(t, s)
	issue, err := s.Create(context.Background(), CreateIssue{Team: "AL", Title: "Keep"})
	if err != nil {
		t.Fatal(err)
	}
	removeSpecSchema(t, root)
	if _, err = root.db.Exec(fmt.Sprintf("PRAGMA user_version=%d", len(migrations)-1)); err != nil {
		t.Fatal(err)
	}
	root.Close()
	root, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	fresh, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if !reflect.DeepEqual(structure(t, root.db), structure(t, fresh.db)) {
		t.Fatal("fresh and upgraded structures differ")
	}
	if _, err = os.Stat(filepath.Join(filepath.Dir(path), "backups", fmt.Sprintf("before-migration-%d.db", len(migrations)))); err != nil {
		t.Fatal(err)
	}
	s, err = root.InWorkspace(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(context.Background(), issue.Identifier); err != nil || !reflect.DeepEqual(got, issue) {
		t.Fatal(got, err)
	}
	if _, err = s.CreateSpec(context.Background(), CreateSpec{Team: "AL", Content: sampleSpec()}); err != nil {
		t.Fatal(err)
	}
}
