package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestProjectTimelineDates(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	p, err := s.CreateProject(ctx, "Timeline", "", "AL")
	if err != nil {
		t.Fatal(err)
	}
	var input UpdateProject
	if err := json.Unmarshal([]byte(`{"start_date":"2026-10-01","target_date":"2026-10-31"}`), &input); err != nil {
		t.Fatal(err)
	}
	input.ID = p.ID
	p, err = s.UpdateProject(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(p)
	var got map[string]any
	json.Unmarshal(data, &got)
	if got["start_date"] != "2026-10-01" || got["target_date"] != "2026-10-31" {
		t.Fatalf("dates not persisted: %s", data)
	}
}

func TestProjectTimelineValidation(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	p, err := s.CreateProject(ctx, "Dates", "", "AL")
	if err != nil {
		t.Fatal(err)
	}
	start, target := "2026-10-01", "2026-10-31"
	if _, err = s.UpdateProject(ctx, UpdateProject{ID: p.ID, StartDate: &start, TargetDate: &target}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"2026-02-30", "2026-1-01", "0000-01-01", "2026-11-01"} {
		if _, err = s.UpdateProject(ctx, UpdateProject{ID: p.ID, StartDate: &bad, Description: new("must rollback")}); !errors.Is(err, ErrProjectDates) {
			t.Fatal(bad, err)
		}
	}
	p, err = s.GetProject(ctx, ProjectRef{ID: p.ID})
	if err != nil || p.Description != "" || *p.StartDate != start {
		t.Fatal(p, err)
	}
	p, err = s.UpdateProject(ctx, UpdateProject{ID: p.ID, StartDate: new("")})
	if err != nil || p.StartDate != nil || p.TargetDate == nil || *p.TargetDate != target {
		t.Fatal(p, err)
	}
	items, err := s.ListProjects(ctx, false)
	if err != nil || len(items) != 1 || !reflect.DeepEqual(items[0], p) {
		t.Fatal(items, err)
	}
	if _, err = s.db.Exec("UPDATE projects SET start_date='2027-01-01' WHERE id=?", p.ID); err == nil {
		t.Fatal("SQL accepted inverted dates")
	}
}

// Historical fixtures built from the current schema must remove later columns
// before pretending to be an earlier version.
func removeTimelineSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS initiative_projects_workspace_insert;
 DROP TRIGGER IF EXISTS initiative_projects_workspace_update;
 DROP TABLE IF EXISTS initiative_projects; DROP TABLE IF EXISTS initiatives;
 ALTER TABLE projects DROP COLUMN target_date; ALTER TABLE projects DROP COLUMN start_date;`); err != nil {
		t.Fatal(err)
	}
}

func TestProjectTimelineUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := inWorkspace(t, root, "default")
	alTeam(t, s)
	ctx := context.Background()
	p, err := s.CreateProject(ctx, "Keep", "Outcome", "AL")
	if err != nil {
		t.Fatal(err)
	}
	cycles, err := s.ConfigureCycles(ctx, ConfigureCycles{Team: "AL", CycleSettings: CycleSettings{Enabled: true, Timezone: "UTC", StartDate: time.Now().UTC().Format(time.DateOnly), DurationWeeks: 2, UpcomingCount: 2}})
	if err != nil {
		t.Fatal(err)
	}
	cycleID := cycles.Cycles[0].ID
	issue, err := s.Create(ctx, CreateIssue{Team: "AL", Title: "Keep issue", ProjectID: new(p.ID), CycleID: &cycleID})
	if err != nil {
		t.Fatal(err)
	}
	cycle, err := s.GetCycle(ctx, cycleID)
	if err != nil {
		t.Fatal(err)
	}
	removeTimelineSchema(t, root.db)
	if _, err = root.db.Exec(fmt.Sprintf("PRAGMA user_version=%d", len(migrations)-2)); err != nil {
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
	if _, err = os.Stat(filepath.Join(filepath.Dir(path), "backups", fmt.Sprintf("before-migration-%d.db", len(migrations)-1))); err != nil {
		t.Fatal(err)
	}
	s, err = root.InWorkspace(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetProject(ctx, ProjectRef{ID: p.ID})
	if err != nil || !reflect.DeepEqual(got, p) {
		t.Fatal(got, err)
	}
	gotIssue, err := s.Get(ctx, issue.Identifier)
	if err != nil || !reflect.DeepEqual(gotIssue, issue) {
		t.Fatal(gotIssue, err)
	}
	gotCycle, err := s.GetCycle(ctx, cycleID)
	if err != nil || !reflect.DeepEqual(gotCycle, cycle) {
		t.Fatal("cycle changed during timeline upgrade", gotCycle, err)
	}
	if _, err = s.CreateInitiative(ctx, CreateInitiative{Name: "Works after upgrade", ProjectIDs: []int64{p.ID}}); err != nil {
		t.Fatal(err)
	}
}
