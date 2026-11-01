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

func TestCycleLifecycle(t *testing.T) {
	s := labelService(t)
	var input CreateIssue
	if err := json.Unmarshal([]byte(`{"team":"AL","title":"Cycle reference","cycle_id":999}`), &input); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(context.Background(), input); err == nil {
		t.Fatal("unknown cycle must be rejected, not silently discarded")
	}
}

func removeCycleSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS issues_cycle_insert; DROP TRIGGER IF EXISTS issues_cycle_update; DROP INDEX IF EXISTS issues_cycle_idx; ALTER TABLE issues DROP COLUMN cycle_id; DROP TABLE cycles; DROP TABLE team_cycle_settings;`); err != nil {
		t.Fatal(err)
	}
}

func TestCycleScheduleAndRollover(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	now := time.Date(2026, 3, 7, 18, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	cfg := ConfigureCycles{Team: "AL", CycleSettings: CycleSettings{Enabled: true, Timezone: "America/New_York", StartDate: "2026-03-07", DurationWeeks: 1, UpcomingCount: 2}}
	page, err := s.ConfigureCycles(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Cycles) != 3 {
		t.Fatal(page)
	}
	first := page.Cycles[2]
	next := page.Cycles[1]
	a, _ := time.Parse(time.RFC3339, first.StartsAt)
	b, _ := time.Parse(time.RFC3339, first.EndsAt)
	if b.Sub(a) != 167*time.Hour {
		t.Fatalf("DST calendar duration=%v", b.Sub(a))
	}
	for _, status := range Statuses {
		if _, err = s.Create(ctx, CreateIssue{Team: "AL", Title: string(status), Status: &status, CycleID: &first.ID}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := s.List(ctx, ListIssues{CycleID: &first.ID})
	if err != nil || len(all) != 8 {
		t.Fatal(all, err)
	}
	now = b
	page, err = s.ListCycles(ctx, "AL")
	if err != nil {
		t.Fatal(err)
	}
	closed, err := s.GetCycle(ctx, first.ID)
	if err != nil || closed.Total != 8 || closed.Done != 1 || closed.Canceled != 1 || closed.RolledOver != 4 || closed.State != "completed" {
		t.Fatal(closed, err)
	}
	remaining, err := s.List(ctx, ListIssues{CycleID: &first.ID})
	if err != nil || len(remaining) != 4 {
		t.Fatal(remaining, err)
	}
	carried, err := s.List(ctx, ListIssues{CycleID: &next.ID})
	if err != nil || len(carried) != 4 {
		t.Fatal(carried, err)
	}
	if _, err = s.Update(ctx, UpdateIssue{Identifier: carried[0].Identifier, CycleID: new(int64(0))}); err != nil {
		t.Fatal(err)
	}
	closedAgain, _ := s.GetCycle(ctx, first.ID)
	if !reflect.DeepEqual(closed, closedAgain) {
		t.Fatal("snapshot changed")
	}
	now = now.AddDate(0, 0, 35)
	for range 2 {
		if _, err = s.ListCycles(ctx, "AL"); err != nil {
			t.Fatal(err)
		}
	}
	page, _ = s.ListCycles(ctx, "AL")
	current := Cycle{}
	for _, c := range page.Cycles {
		if c.State == "current" {
			current = c
		}
	}
	carried, err = s.List(ctx, ListIssues{CycleID: &current.ID})
	if err != nil || len(carried) != 3 {
		t.Fatal(carried, err)
	}
	if _, err = s.Create(ctx, CreateIssue{Team: "AL", Title: "closed", CycleID: &first.ID}); !errors.Is(err, ErrCycleClosed) {
		t.Fatal(err)
	}
	cfg.Enabled = false
	if _, err = s.ConfigureCycles(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	page, _ = s.ListCycles(ctx, "AL")
	for _, c := range page.Cycles {
		if c.State != "completed" {
			t.Fatal(c)
		}
	}
}

func TestCycleAuthorization(t *testing.T) {
	root, s, admin, member := accessFixture(t)
	_, err := s.CreateTeam(admin, CreateTeam{Key: "AL", Name: "Private", Private: true})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	cfg := ConfigureCycles{Team: "AL", CycleSettings: CycleSettings{Enabled: true, Timezone: "UTC", StartDate: "2026-10-14", DurationWeeks: 2, UpcomingCount: 2}}
	page, err := s.ConfigureCycles(admin, cfg)
	if err != nil {
		t.Fatal(err)
	}
	id := page.Cycles[0].ID
	if _, err = s.GetCycle(member, id); !errors.Is(err, ErrCycleNotFound) {
		t.Fatal(err)
	}
	if _, err = s.ListCycles(member, "AL"); !errors.Is(err, ErrTeamNotFound) {
		t.Fatal(err)
	}
	if _, err = root.CreateWorkspace(admin, Workspace{Slug: "other", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	other, err := root.InWorkspace(admin, "other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = other.CreateTeam(admin, CreateTeam{Key: "AL", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	if _, err = other.GetCycle(admin, id); !errors.Is(err, ErrCycleNotFound) {
		t.Fatal(err)
	}
	if _, err = other.Create(admin, CreateIssue{Team: "AL", Title: "cross workspace", CycleID: &id}); !errors.Is(err, ErrCycleNotFound) {
		t.Fatal(err)
	}
	if _, err = s.CreateTeam(admin, CreateTeam{Key: "BB", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(admin, CreateIssue{Team: "BB", Title: "cross team", CycleID: &id}); !errors.Is(err, ErrCycleNotFound) {
		t.Fatal(err)
	}
	cfg.Team = "BB"
	if _, err = s.ConfigureCycles(member, cfg); !errors.Is(err, ErrTeamOwnerRequired) {
		t.Fatal(err)
	}
	issue, err := s.Create(admin, CreateIssue{Team: "BB", Title: "SQL constraint"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`UPDATE issues SET cycle_id=? WHERE identifier=?`, id, issue.Identifier); err == nil {
		t.Fatal("SQL accepted mismatched team")
	}
}

func TestCycleUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := inWorkspace(t, root, "default")
	alTeam(t, s)
	before, err := s.Create(context.Background(), CreateIssue{Team: "AL", Title: "preserve", Description: "body"})
	if err != nil {
		t.Fatal(err)
	}
	removeCycleSchema(t, root.db)
	removeTimelineSchema(t, root.db)
	if _, err = root.db.Exec(fmt.Sprintf("PRAGMA user_version=%d", len(migrations)-3)); err != nil {
		t.Fatal(err)
	}
	root.Close()
	root, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	s, err = root.InWorkspace(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	after, err := s.Get(context.Background(), before.Identifier)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal(before, after, err)
	}
	if _, err = os.Stat(filepath.Join(filepath.Dir(path), "backups", fmt.Sprintf("before-migration-%d.db", len(migrations)-2))); err != nil {
		t.Fatal(err)
	}
	fresh, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if !reflect.DeepEqual(structure(t, root.db), structure(t, fresh.db)) {
		t.Fatal("schema differs")
	}
}

func TestCycleValidationAndAtomicity(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	cfg := ConfigureCycles{Team: "AL", CycleSettings: CycleSettings{Enabled: true, Timezone: "UTC", StartDate: "2026-10-14", DurationWeeks: 2, UpcomingCount: 2}}
	for _, change := range []func(*ConfigureCycles){func(c *ConfigureCycles) { c.Timezone = "Local" }, func(c *ConfigureCycles) { c.Timezone = "bad" }, func(c *ConfigureCycles) { c.StartDate = "2026-10-13" }, func(c *ConfigureCycles) { c.StartDate = "2026-02-30" }, func(c *ConfigureCycles) { c.DurationWeeks = 0 }, func(c *ConfigureCycles) { c.UpcomingCount = 16 }} {
		bad := cfg
		change(&bad)
		if _, err := s.ConfigureCycles(ctx, bad); !errors.Is(err, ErrCycleSettings) {
			t.Fatal(bad, err)
		}
	}
	page, err := s.ConfigureCycles(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	id := page.Cycles[2].ID
	bad := cfg
	bad.DurationWeeks = 1
	if _, err = s.ConfigureCycles(ctx, bad); !errors.Is(err, ErrCycleSchedule) {
		t.Fatal(err)
	}
	issue, err := s.Create(ctx, CreateIssue{Team: "AL", Title: "Atomic", Status: new(StatusTodo), CycleID: &id})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(ctx, UpdateIssue{Identifier: issue.Identifier, Title: new("must roll back"), CycleID: new(int64(9999))}); err == nil {
		t.Fatal("invalid update succeeded")
	}
	got, err := s.Get(ctx, issue.Identifier)
	if err != nil || got.Title != "Atomic" {
		t.Fatal(got, err)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER fail_roll BEFORE UPDATE OF cycle_id ON issues BEGIN SELECT RAISE(ABORT,'fail rollover'); END`); err != nil {
		t.Fatal(err)
	}
	now = now.AddDate(0, 0, 15)
	if _, err = s.ListCycles(ctx, "AL"); err == nil {
		t.Fatal("expected transaction failure")
	}
	var completed sql.NullString
	if err = s.db.QueryRow(`SELECT completed_at FROM cycles WHERE id=?`, id).Scan(&completed); err != nil || completed.Valid {
		t.Fatal(completed, err)
	}
	if _, err = s.db.Exec(`DROP TRIGGER fail_roll`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ListCycles(ctx, "AL"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(ctx, UpdateIssue{Identifier: issue.Identifier, CycleID: new(int64(-1))}); !errors.Is(err, ErrCycleReference) {
		t.Fatal(err)
	}
}

func TestCycleRestartAndDisable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := inWorkspace(t, root, "default")
	alTeam(t, s)
	ctx := context.Background()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	root.now = func() time.Time { return now }
	s.now = root.now
	cfg := ConfigureCycles{Team: "AL", CycleSettings: CycleSettings{Enabled: true, Timezone: "UTC", StartDate: "2026-10-14", DurationWeeks: 1, UpcomingCount: 2}}
	page, err := s.ConfigureCycles(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	original := page.Cycles[2].ID
	issue, err := s.Create(ctx, CreateIssue{Team: "AL", Title: "Restart", Status: new(StatusInProgress), CycleID: &original})
	if err != nil {
		t.Fatal(err)
	}
	root.Close()
	now = now.AddDate(0, 0, 22)
	root, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	root.now = func() time.Time { return now }
	for range 2 {
		if err = root.AdvanceCycles(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s, err = root.InWorkspace(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, issue.Identifier)
	if err != nil || got.CycleID == nil || *got.CycleID == original || got.Status != StatusInProgress {
		t.Fatal(got, err)
	}
	page, err = s.ListCycles(ctx, "AL")
	if err != nil {
		t.Fatal(err)
	}
	future := page.Cycles[0].ID
	planned, err := s.Create(ctx, CreateIssue{Team: "AL", Title: "Future", CycleID: &future})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Enabled = false
	if _, err = s.ConfigureCycles(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get(ctx, planned.Identifier)
	if err != nil || got.CycleID != nil {
		t.Fatal(got, err)
	}
	if _, err = s.GetCycle(ctx, future); !errors.Is(err, ErrCycleNotFound) {
		t.Fatal(err)
	}
	cfg.Enabled = true
	cfg.StartDate = now.AddDate(0, 0, 1).Format(time.DateOnly)
	cfg.Timezone = "Asia/Shanghai"
	if _, err = s.ConfigureCycles(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	old, err := s.GetCycle(ctx, original)
	if err != nil || old.Timezone != "UTC" {
		t.Fatal(old, err)
	}
}
