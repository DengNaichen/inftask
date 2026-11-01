package app

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// migrations upgrade an existing database one version at a time, and PRAGMA
// user_version counts how many have run. schema.sql always describes the
// current shape, so a new database starts at len(migrations) without running
// any. Append only; never edit a migration that has shipped.
var migrations = []string{
	// 1: teams. Every identifier so far is <prefix>-<id>, so each prefix
	// becomes a team, the id after the last dash becomes the issue's number,
	// and the team's counter continues after its highest number. Rebuilding
	// issues keeps ids, which events reference, and identifiers, which
	// everything else references.
	`CREATE TABLE teams (
		key TEXT PRIMARY KEY NOT NULL,
		name TEXT NOT NULL,
		next_number INTEGER NOT NULL
	);
	CREATE TABLE issues_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		identifier TEXT NOT NULL UNIQUE,
		team TEXT NOT NULL REFERENCES teams(key),
		number INTEGER NOT NULL,
		title TEXT NOT NULL CHECK (length(trim(title)) > 0),
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL CHECK (status IN ('triage', 'todo', 'agent_ready', 'in_progress', 'in_review', 'done', 'canceled')),
		type TEXT CHECK (type IN ('spec', 'feature', 'bug', 'improvement', 'chore')),
		project TEXT REFERENCES projects(name),
		parent TEXT REFERENCES issues(identifier) CHECK (parent <> identifier),
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	INSERT INTO issues_new(id, identifier, team, number, title, description, status, type, project, parent, created_at, updated_at)
	SELECT id, identifier, substr(head, 1, length(head) - 1), CAST(substr(identifier, length(head) + 1) AS INTEGER),
		title, description, status, type, project, parent, created_at, updated_at
	FROM (SELECT *, rtrim(identifier, '0123456789') AS head FROM issues);
	INSERT INTO teams(key, name, next_number) SELECT team, team, max(number) + 1 FROM issues_new GROUP BY team;
	DROP TABLE issues;
	ALTER TABLE issues_new RENAME TO issues;
	CREATE UNIQUE INDEX issues_team_number_idx ON issues(team, number);
	CREATE INDEX issues_status_updated_idx ON issues(status, updated_at DESC);
	CREATE INDEX issues_parent_idx ON issues(parent);`,

	// 2: references by id. Team keys are to become unique only within a
	// workspace, so tables reference teams, projects, and issues by integer
	// id. Issues, links, and comments keep their ids; teams and projects are
	// numbered in creation order. Related rows kept issue < other as text,
	// where 'AL-10' < 'AL-9', so they are reordered by id.
	`CREATE TABLE teams_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		next_number INTEGER NOT NULL
	);
	INSERT INTO teams_new(key, name, next_number) SELECT key, name, next_number FROM teams ORDER BY rowid;
	CREATE TABLE projects_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1))
	);
	INSERT INTO projects_new(name, description, archived) SELECT name, description, archived FROM projects ORDER BY rowid;
	CREATE TABLE issues_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		identifier TEXT NOT NULL UNIQUE,
		team_id INTEGER NOT NULL REFERENCES teams(id),
		number INTEGER NOT NULL,
		title TEXT NOT NULL CHECK (length(trim(title)) > 0),
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL CHECK (status IN ('triage', 'todo', 'agent_ready', 'in_progress', 'in_review', 'done', 'canceled')),
		type TEXT CHECK (type IN ('spec', 'feature', 'bug', 'improvement', 'chore')),
		project_id INTEGER REFERENCES projects(id),
		parent_id INTEGER REFERENCES issues(id) CHECK (parent_id <> id),
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	INSERT INTO issues_new(id, identifier, team_id, number, title, description, status, type, project_id, parent_id, created_at, updated_at)
	SELECT id, identifier, (SELECT id FROM teams_new WHERE key = issues.team), number, title, description, status, type,
		(SELECT id FROM projects_new WHERE name = issues.project), (SELECT p.id FROM issues p WHERE p.identifier = issues.parent),
		created_at, updated_at
	FROM issues;
	CREATE TABLE relations_new (
		issue_id INTEGER NOT NULL REFERENCES issues(id),
		kind TEXT NOT NULL CHECK (kind IN ('blocks', 'related')),
		other_id INTEGER NOT NULL REFERENCES issues(id),
		PRIMARY KEY (issue_id, kind, other_id),
		CHECK (issue_id <> other_id),
		CHECK (kind <> 'related' OR issue_id < other_id)
	);
	INSERT INTO relations_new(issue_id, kind, other_id)
	SELECT CASE kind WHEN 'related' THEN min(issue_id, other_id) ELSE issue_id END, kind,
		CASE kind WHEN 'related' THEN max(issue_id, other_id) ELSE other_id END
	FROM (SELECT (SELECT id FROM issues WHERE identifier = relations.issue) AS issue_id, kind,
		(SELECT id FROM issues WHERE identifier = relations.other) AS other_id FROM relations);
	CREATE TABLE links_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		issue_id INTEGER NOT NULL REFERENCES issues(id),
		title TEXT NOT NULL,
		url TEXT NOT NULL
	);
	INSERT INTO links_new(id, issue_id, title, url)
	SELECT id, (SELECT id FROM issues WHERE identifier = links.issue), title, url FROM links;
	CREATE TABLE comments_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		issue_id INTEGER NOT NULL REFERENCES issues(id),
		body TEXT NOT NULL,
		created_at TEXT NOT NULL
	);
	INSERT INTO comments_new(id, issue_id, body, created_at)
	SELECT id, (SELECT id FROM issues WHERE identifier = comments.issue), body, created_at FROM comments;
	CREATE TABLE issue_labels_new (
		issue_id INTEGER NOT NULL REFERENCES issues(id),
		label_id INTEGER NOT NULL REFERENCES labels(id),
		PRIMARY KEY (issue_id, label_id)
	);
	INSERT INTO issue_labels_new(issue_id, label_id)
	SELECT (SELECT id FROM issues WHERE identifier = issue_labels.issue), label_id FROM issue_labels;
	DROP TABLE issue_labels;
	DROP TABLE comments;
	DROP TABLE links;
	DROP TABLE relations;
	DROP TABLE issues;
	DROP TABLE projects;
	DROP TABLE teams;
	ALTER TABLE teams_new RENAME TO teams;
	ALTER TABLE projects_new RENAME TO projects;
	ALTER TABLE issues_new RENAME TO issues;
	ALTER TABLE relations_new RENAME TO relations;
	ALTER TABLE links_new RENAME TO links;
	ALTER TABLE comments_new RENAME TO comments;
	ALTER TABLE issue_labels_new RENAME TO issue_labels;
	CREATE UNIQUE INDEX issues_team_number_idx ON issues(team_id, number);
	CREATE INDEX issues_status_updated_idx ON issues(status, updated_at DESC);
	CREATE INDEX issues_parent_idx ON issues(parent_id);
	CREATE INDEX issue_labels_label_idx ON issue_labels(label_id, issue_id);
	CREATE TRIGGER issue_labels_exclusive BEFORE INSERT ON issue_labels
	WHEN EXISTS (
		SELECT 1 FROM issue_labels il JOIN labels old ON old.id = il.label_id
		JOIN labels incoming ON incoming.id = NEW.label_id
		WHERE il.issue_id = NEW.issue_id AND old.group_id = incoming.group_id AND old.id <> incoming.id
	)
	BEGIN SELECT RAISE(ABORT, 'label group is single-select'); END;`,

	// 3: workspaces. Team keys, Project names, label group and label names,
	// and issue identifiers become unique within a workspace, so those tables
	// and the pull request records gain workspace_id. Everything so far moves
	// into workspace default. Every row keeps its id. The trigger is dropped
	// first because it names labels, which is rebuilt.
	`CREATE TABLE workspaces (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL
	);
	INSERT INTO workspaces(slug, name) VALUES ('default', 'Default');
	DROP TRIGGER issue_labels_exclusive;
	CREATE TABLE projects_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1)),
		UNIQUE (workspace_id, name)
	);
	INSERT INTO projects_new(id, workspace_id, name, description, archived)
	SELECT id, (SELECT id FROM workspaces), name, description, archived FROM projects;
	CREATE TABLE teams_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		key TEXT NOT NULL,
		name TEXT NOT NULL,
		next_number INTEGER NOT NULL,
		UNIQUE (workspace_id, key)
	);
	INSERT INTO teams_new(id, workspace_id, key, name, next_number)
	SELECT id, (SELECT id FROM workspaces), key, name, next_number FROM teams;
	CREATE TABLE issues_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		identifier TEXT NOT NULL,
		team_id INTEGER NOT NULL REFERENCES teams(id),
		number INTEGER NOT NULL,
		title TEXT NOT NULL CHECK (length(trim(title)) > 0),
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL CHECK (status IN ('triage', 'todo', 'agent_ready', 'in_progress', 'in_review', 'done', 'canceled')),
		type TEXT CHECK (type IN ('spec', 'feature', 'bug', 'improvement', 'chore')),
		project_id INTEGER REFERENCES projects(id),
		parent_id INTEGER REFERENCES issues(id) CHECK (parent_id <> id),
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		UNIQUE (workspace_id, identifier)
	);
	INSERT INTO issues_new(id, workspace_id, identifier, team_id, number, title, description, status, type, project_id, parent_id, created_at, updated_at)
	SELECT id, (SELECT id FROM workspaces), identifier, team_id, number, title, description, status, type, project_id, parent_id, created_at, updated_at
	FROM issues;
	CREATE TABLE pull_requests_new (
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		repo TEXT NOT NULL,
		number INTEGER NOT NULL,
		title TEXT NOT NULL,
		url TEXT NOT NULL,
		head_ref TEXT NOT NULL,
		state TEXT NOT NULL CHECK (state IN ('open', 'draft', 'merged', 'closed')),
		issue_identifier TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		linkback_posted_at TEXT,
		PRIMARY KEY (workspace_id, repo, number, issue_identifier)
	);
	INSERT INTO pull_requests_new(workspace_id, repo, number, title, url, head_ref, state, issue_identifier, updated_at, linkback_posted_at)
	SELECT (SELECT id FROM workspaces), repo, number, title, url, head_ref, state, issue_identifier, updated_at, linkback_posted_at
	FROM pull_requests;
	CREATE TABLE github_sync_new (
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		repo TEXT NOT NULL,
		last_updated_at TEXT NOT NULL,
		PRIMARY KEY (workspace_id, repo)
	);
	INSERT INTO github_sync_new(workspace_id, repo, last_updated_at)
	SELECT (SELECT id FROM workspaces), repo, last_updated_at FROM github_sync;
	CREATE TABLE label_groups_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		name TEXT NOT NULL CHECK (length(trim(name)) > 0),
		name_key TEXT NOT NULL,
		UNIQUE (workspace_id, name_key),
		UNIQUE (workspace_id, id)
	);
	INSERT INTO label_groups_new(id, workspace_id, name, name_key)
	SELECT id, (SELECT id FROM workspaces), name, name_key FROM label_groups;
	CREATE TABLE labels_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		name TEXT NOT NULL CHECK (length(trim(name)) > 0),
		name_key TEXT NOT NULL,
		color TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		group_id INTEGER,
		archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1)),
		FOREIGN KEY (workspace_id, group_id) REFERENCES label_groups(workspace_id, id)
	);
	INSERT INTO labels_new(id, workspace_id, name, name_key, color, description, group_id, archived)
	SELECT id, (SELECT id FROM workspaces), name, name_key, color, description, group_id, archived FROM labels;
	DROP TABLE labels;
	DROP TABLE label_groups;
	DROP TABLE github_sync;
	DROP TABLE pull_requests;
	DROP TABLE issues;
	DROP TABLE teams;
	DROP TABLE projects;
	ALTER TABLE projects_new RENAME TO projects;
	ALTER TABLE teams_new RENAME TO teams;
	ALTER TABLE issues_new RENAME TO issues;
	ALTER TABLE pull_requests_new RENAME TO pull_requests;
	ALTER TABLE github_sync_new RENAME TO github_sync;
	ALTER TABLE label_groups_new RENAME TO label_groups;
	ALTER TABLE labels_new RENAME TO labels;
	CREATE UNIQUE INDEX issues_team_number_idx ON issues(team_id, number);
	CREATE INDEX issues_status_updated_idx ON issues(status, updated_at DESC);
	CREATE INDEX issues_parent_idx ON issues(parent_id);
	CREATE UNIQUE INDEX labels_name_idx ON labels(workspace_id, COALESCE(group_id, 0), name_key);
	CREATE TRIGGER issue_labels_exclusive BEFORE INSERT ON issue_labels
	WHEN EXISTS (
		SELECT 1 FROM issue_labels il JOIN labels old ON old.id = il.label_id
		JOIN labels incoming ON incoming.id = NEW.label_id
		WHERE il.issue_id = NEW.issue_id AND old.group_id = incoming.group_id AND old.id <> incoming.id
	)
	BEGIN SELECT RAISE(ABORT, 'label group is single-select'); END;`,

	// 4: backlog. Rebuild only issues to extend its status CHECK; every id,
	// workspace, field, reference, and the autoincrement high-water mark stays.
	`CREATE TABLE issues_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		identifier TEXT NOT NULL,
		team_id INTEGER NOT NULL REFERENCES teams(id),
		number INTEGER NOT NULL,
		title TEXT NOT NULL CHECK (length(trim(title)) > 0),
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL CHECK (status IN ('triage', 'backlog', 'todo', 'agent_ready', 'in_progress', 'in_review', 'done', 'canceled')),
		type TEXT CHECK (type IN ('spec', 'feature', 'bug', 'improvement', 'chore')),
		project_id INTEGER REFERENCES projects(id),
		parent_id INTEGER REFERENCES issues(id) CHECK (parent_id <> id),
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		UNIQUE (workspace_id, identifier)
	);
	INSERT INTO issues_new(id, workspace_id, identifier, team_id, number, title, description, status, type, project_id, parent_id, created_at, updated_at)
	SELECT id, workspace_id, identifier, team_id, number, title, description, status, type, project_id, parent_id, created_at, updated_at FROM issues;
	DELETE FROM sqlite_sequence WHERE name = 'issues_new';
	INSERT INTO sqlite_sequence(name, seq) SELECT 'issues_new', seq FROM sqlite_sequence WHERE name = 'issues';
	DROP TABLE issues;
	ALTER TABLE issues_new RENAME TO issues;
	CREATE UNIQUE INDEX issues_team_number_idx ON issues(team_id, number);
	CREATE INDEX issues_status_updated_idx ON issues(status, updated_at DESC);
	CREATE INDEX issues_parent_idx ON issues(parent_id);`,

	// 5: Linear-compatible priority. Existing issues retain their data and
	// receive no priority; adding a column does not rebuild references.
	`ALTER TABLE issues ADD COLUMN priority INTEGER NOT NULL DEFAULT 0 CHECK (priority IN (0, 1, 2, 3, 4));`,

	// 6: comment authors. Comments imported from Linear keep who wrote them;
	// existing comments, like those added later, have no author.
	`ALTER TABLE comments ADD COLUMN author TEXT NOT NULL DEFAULT '';`,

	// Entity emoji icons. Empty values keep the existing default icons.
	`ALTER TABLE projects ADD COLUMN emoji TEXT NOT NULL DEFAULT '';
	ALTER TABLE teams ADD COLUMN emoji TEXT NOT NULL DEFAULT '';`,
	// Explicit Project ownership, inferred only from existing issue facts.
	`CREATE TABLE IF NOT EXISTS project_teams (
    project_id INTEGER NOT NULL REFERENCES projects(id),
    team_id INTEGER NOT NULL REFERENCES teams(id),
    PRIMARY KEY (project_id, team_id)
);
CREATE INDEX IF NOT EXISTS project_teams_team_idx ON project_teams(team_id, project_id);
CREATE TRIGGER IF NOT EXISTS project_teams_workspace BEFORE INSERT ON project_teams
WHEN (SELECT workspace_id FROM projects WHERE id=NEW.project_id) <> (SELECT workspace_id FROM teams WHERE id=NEW.team_id)
BEGIN SELECT RAISE(ABORT, 'project and team must belong to the same workspace'); END;
INSERT INTO project_teams(project_id,team_id) SELECT DISTINCT project_id,team_id FROM issues WHERE project_id IS NOT NULL;`,
	// Workspace membership; existing workspaces require explicit bootstrap.
	`
CREATE TABLE IF NOT EXISTS users (
 subject TEXT PRIMARY KEY NOT NULL,
 name TEXT NOT NULL,
 email TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS workspace_members (
 workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
 subject TEXT NOT NULL REFERENCES users(subject),
 role TEXT NOT NULL CHECK(role IN ('admin','member')),
 status TEXT NOT NULL CHECK(status IN ('active','suspended')),
 PRIMARY KEY(workspace_id,subject)
);
CREATE INDEX IF NOT EXISTS workspace_members_subject_idx ON workspace_members(subject,workspace_id);
CREATE TABLE IF NOT EXISTS workspace_invites (
 workspace_id INTEGER PRIMARY KEY REFERENCES workspaces(id),
 digest BLOB NOT NULL UNIQUE
);
`,
	// Team membership and private visibility; existing teams remain public.
	`ALTER TABLE teams ADD COLUMN private BOOLEAN NOT NULL DEFAULT FALSE CHECK(private IN (0,1));

CREATE TABLE IF NOT EXISTS team_members (
 team_id INTEGER NOT NULL REFERENCES teams(id),
 subject TEXT NOT NULL REFERENCES users(subject),
 role TEXT NOT NULL CHECK(role IN ('member','owner')),
 PRIMARY KEY(team_id,subject)
);
CREATE TRIGGER IF NOT EXISTS team_members_workspace BEFORE INSERT ON team_members
WHEN NOT EXISTS(SELECT 1 FROM workspace_members m JOIN teams t ON t.workspace_id=m.workspace_id WHERE t.id=NEW.team_id AND m.subject=NEW.subject)
BEGIN SELECT RAISE(ABORT, 'team member must belong to workspace'); END;
`,
	// Human assignment and its trusted actor history.
	`ALTER TABLE issues ADD COLUMN assignee_subject TEXT REFERENCES users(subject);
CREATE TABLE IF NOT EXISTS issue_assignments (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
 actor_subject TEXT REFERENCES users(subject),
 from_subject TEXT REFERENCES users(subject),
 to_subject TEXT REFERENCES users(subject),
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS issue_assignments_issue_idx ON issue_assignments(issue_id,id);
CREATE INDEX IF NOT EXISTS issues_assignee_idx ON issues(workspace_id,assignee_subject);
`,
	// Explicit agent delegation, separate from status and human assignment.
	`CREATE TABLE IF NOT EXISTS issue_delegations (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
 agent TEXT NOT NULL CHECK(agent IN ('codex','claude')),
 session_id TEXT NOT NULL CHECK(length(session_id) BETWEEN 1 AND 256),
 agent_id TEXT NOT NULL DEFAULT '' CHECK(length(agent_id) <= 256),
 agent_type TEXT NOT NULL DEFAULT '' CHECK(length(agent_type) <= 128),
 hostname TEXT NOT NULL CHECK(length(hostname) BETWEEN 1 AND 253),
 actor_subject TEXT REFERENCES users(subject),
 created_at TEXT NOT NULL,
 UNIQUE(issue_id,agent,session_id,agent_id,hostname)
);
CREATE INDEX IF NOT EXISTS issue_delegations_issue_idx ON issue_delegations(issue_id,id);
`,
	// Separate agent work records and decision summaries from comments.
	`CREATE TABLE IF NOT EXISTS work_logs (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('progress','decision')),
 summary TEXT NOT NULL CHECK(length(summary) BETWEEN 1 AND 240),
 body TEXT NOT NULL CHECK(length(CAST(body AS BLOB)) BETWEEN 1 AND 65536),
 actor_subject TEXT REFERENCES users(subject),
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS work_logs_issue_idx ON work_logs(issue_id,id);
CREATE INDEX IF NOT EXISTS work_logs_decisions_idx ON work_logs(issue_id,id) WHERE kind='decision';
`,
	// Enforce a DAG for dependencies and reject legacy cycles without deleting edges.
	`CREATE TRIGGER IF NOT EXISTS relations_blocks_insert AFTER INSERT ON relations
WHEN NEW.kind = 'blocks'
BEGIN
    SELECT RAISE(ABORT, 'dependency cycle is not allowed')
    WHERE EXISTS (
        WITH RECURSIVE reachable(id) AS (
            SELECT NEW.other_id
            UNION
            SELECT r.other_id FROM relations r JOIN reachable p ON r.issue_id = p.id
            WHERE r.kind = 'blocks'
        )
        SELECT 1 FROM reachable WHERE id = NEW.issue_id
    );
END;

CREATE TRIGGER IF NOT EXISTS relations_blocks_update AFTER UPDATE OF issue_id, kind, other_id ON relations
WHEN NEW.kind = 'blocks'
BEGIN
    SELECT RAISE(ABORT, 'dependency cycle is not allowed')
    WHERE EXISTS (
        WITH RECURSIVE reachable(id) AS (
            SELECT NEW.other_id
            UNION
            SELECT r.other_id FROM relations r JOIN reachable p ON r.issue_id = p.id
            WHERE r.kind = 'blocks'
        )
        SELECT 1 FROM reachable WHERE id = NEW.issue_id
    );
END;

UPDATE relations SET kind = kind WHERE kind = 'blocks';
`,
	// Add team cycles without changing existing issue values.
	`CREATE TABLE IF NOT EXISTS team_cycle_settings (
 team_id INTEGER PRIMARY KEY REFERENCES teams(id),
 enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
 timezone TEXT NOT NULL,
 start_date TEXT NOT NULL,
 duration_weeks INTEGER NOT NULL CHECK(duration_weeks BETWEEN 1 AND 8),
 upcoming_count INTEGER NOT NULL CHECK(upcoming_count BETWEEN 1 AND 15)
);
CREATE TABLE IF NOT EXISTS cycles (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 team_id INTEGER NOT NULL REFERENCES teams(id),
 number INTEGER NOT NULL,
 name TEXT NOT NULL DEFAULT '',
 description TEXT NOT NULL DEFAULT '',
 timezone TEXT NOT NULL,
 starts_at TEXT NOT NULL,
 ends_at TEXT NOT NULL,
 completed_at TEXT,
 total INTEGER NOT NULL DEFAULT 0,
 done INTEGER NOT NULL DEFAULT 0,
 canceled INTEGER NOT NULL DEFAULT 0,
 rolled_over INTEGER NOT NULL DEFAULT 0,
 UNIQUE(team_id,number),
 CHECK(starts_at < ends_at)
);
CREATE INDEX IF NOT EXISTS cycles_team_dates_idx ON cycles(team_id,starts_at);
ALTER TABLE issues ADD COLUMN cycle_id INTEGER REFERENCES cycles(id);
CREATE INDEX IF NOT EXISTS issues_cycle_idx ON issues(cycle_id);
CREATE TRIGGER IF NOT EXISTS issues_cycle_insert BEFORE INSERT ON issues
WHEN NEW.cycle_id IS NOT NULL
BEGIN
 SELECT RAISE(ABORT, 'cycle must belong to the issue team') WHERE NOT EXISTS(SELECT 1 FROM cycles WHERE id=NEW.cycle_id AND team_id=NEW.team_id);
END;
CREATE TRIGGER IF NOT EXISTS issues_cycle_update BEFORE UPDATE OF cycle_id,team_id ON issues
WHEN NEW.cycle_id IS NOT NULL
BEGIN
 SELECT RAISE(ABORT, 'cycle must belong to the issue team') WHERE NOT EXISTS(SELECT 1 FROM cycles WHERE id=NEW.cycle_id AND team_id=NEW.team_id);
END;
`,
	`ALTER TABLE projects ADD COLUMN start_date TEXT;
ALTER TABLE projects ADD COLUMN target_date TEXT CHECK(start_date IS NULL OR target_date IS NULL OR start_date <= target_date);

CREATE TABLE IF NOT EXISTS initiatives (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
    name TEXT NOT NULL CHECK(length(trim(name)) > 0),
    description TEXT NOT NULL DEFAULT '',
    archived BOOLEAN NOT NULL DEFAULT FALSE CHECK(archived IN (0,1)),
    UNIQUE(workspace_id,name)
);
CREATE TABLE IF NOT EXISTS initiative_projects (
    initiative_id INTEGER NOT NULL REFERENCES initiatives(id),
    project_id INTEGER NOT NULL REFERENCES projects(id),
    PRIMARY KEY(initiative_id,project_id)
);
CREATE INDEX IF NOT EXISTS initiative_projects_project ON initiative_projects(project_id);
CREATE TRIGGER IF NOT EXISTS initiative_projects_workspace_insert BEFORE INSERT ON initiative_projects
WHEN (SELECT workspace_id FROM initiatives WHERE id=NEW.initiative_id) <> (SELECT workspace_id FROM projects WHERE id=NEW.project_id)
BEGIN SELECT RAISE(ABORT, 'initiative and project must share a workspace'); END;
CREATE TRIGGER IF NOT EXISTS initiative_projects_workspace_update BEFORE UPDATE ON initiative_projects
WHEN (SELECT workspace_id FROM initiatives WHERE id=NEW.initiative_id) <> (SELECT workspace_id FROM projects WHERE id=NEW.project_id)
BEGIN SELECT RAISE(ABORT, 'initiative and project must share a workspace'); END;
`,
	// Specs and their append-only structured revisions.
	`CREATE TABLE IF NOT EXISTS specs (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
 team_id INTEGER NOT NULL REFERENCES teams(id),
 number INTEGER NOT NULL CHECK(number > 0),
 current_revision INTEGER NOT NULL CHECK(current_revision > 0),
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 UNIQUE(workspace_id,number)
);
CREATE INDEX IF NOT EXISTS specs_team_idx ON specs(team_id);
CREATE TRIGGER IF NOT EXISTS specs_workspace_insert BEFORE INSERT ON specs
WHEN (SELECT workspace_id FROM teams WHERE id=NEW.team_id) <> NEW.workspace_id
BEGIN SELECT RAISE(ABORT, 'spec and team must share a workspace'); END;
CREATE TRIGGER IF NOT EXISTS specs_workspace_update BEFORE UPDATE OF workspace_id,team_id ON specs
WHEN (SELECT workspace_id FROM teams WHERE id=NEW.team_id) <> NEW.workspace_id
BEGIN SELECT RAISE(ABORT, 'spec and team must share a workspace'); END;
-- Each save appends a full structured snapshot; revisions are never edited or removed.
CREATE TABLE IF NOT EXISTS spec_revisions (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 spec_id INTEGER NOT NULL REFERENCES specs(id),
 number INTEGER NOT NULL CHECK(number > 0),
 content TEXT NOT NULL CHECK(json_valid(content)),
 message TEXT NOT NULL DEFAULT '',
 author_subject TEXT REFERENCES users(subject),
 created_at TEXT NOT NULL,
 UNIQUE(spec_id,number)
);
CREATE TRIGGER IF NOT EXISTS spec_revisions_no_update BEFORE UPDATE ON spec_revisions
BEGIN SELECT RAISE(ABORT, 'spec revisions are append-only'); END;
CREATE TRIGGER IF NOT EXISTS spec_revisions_no_delete BEFORE DELETE ON spec_revisions
BEGIN SELECT RAISE(ABORT, 'spec revisions are append-only'); END;
`,
}

func migrate(db *sql.DB, backupDir string, migrations []string) error {
	var tables int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'issues'`).Scan(&tables); err != nil {
		return err
	}
	if tables == 0 {
		if _, err := db.Exec(schema); err != nil {
			return err
		}
		_, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations)))
		return err
	}

	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version < len(migrations) {
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return err
		}
		if _, err := db.Exec(`VACUUM INTO ?`, filepath.Join(backupDir, fmt.Sprintf("before-migration-%d.db", version+1))); err != nil {
			return err
		}
		// Rebuilding a table, which changing a CHECK constraint requires, only
		// works with foreign keys off outside the transaction.
		if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
			return err
		}
		for ; version < len(migrations); version++ {
			if err := runMigration(db, migrations[version], version+1); err != nil {
				return fmt.Errorf("migration %d: %w", version+1, err)
			}
		}
		if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
			return err
		}
	}
	_, err := db.Exec(schema)
	return err
}

func runMigration(db *sql.DB, statements string, version int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(statements); err != nil {
		return err
	}
	rows, err := tx.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	violated := rows.Next()
	rows.Close()
	if violated {
		return errors.New("leaves foreign key violations")
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
		return err
	}
	return tx.Commit()
}
