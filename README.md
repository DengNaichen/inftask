# InfTask

An agent-first delivery board for local use or a shared cloud instance: one Go process, one SQLite file, one React Kanban.

## Run

```sh
npm --prefix web install
npm --prefix web run build
go run ./cmd/inftask
```

Open <http://localhost:8787>. Data is stored in `inftask.db` by default.

Teams, Projects, and labels belong to a workspace, and a workspace's slug names it in URLs: its pages are under `/<slug>/`, its REST routes under `/api/w/<slug>/`, and its MCP endpoint is `/w/<slug>/mcp`. `/` opens the first workspace by slug. A new database has no workspace, and the page says so until you create one with `POST /api/workspaces`:

```sh
curl -X POST http://127.0.0.1:8787/api/workspaces -H 'Content-Type: application/json' -d '{"slug":"default","name":"Default"}'
```

Use the current workspace name at the top left to switch workspaces, choose **Create workspace…** (slug and name), or **Rename workspace…** (name only). Creating opens the new workspace; switching opens its issue list and resets the previous workspace’s filters. Failed saves keep the form inputs and show an error so you can retry. The menu remains available on narrow screens.

A slug is 2 to 32 lowercase letters, digits, or hyphens starting with a letter, other than `api`, `assets`, `auth`, `mcp`, `w`, and `webhooks`; it cannot be changed. `GET /api/workspaces` lists workspaces by slug, and `PATCH /api/workspaces/<slug>` with `{"name": ...}` renames one. An unknown slug returns 404. Workspaces only group data: there are no members or roles, and they cannot be deleted.

Every issue belongs to a team, and each team numbers its own issues: team `AL` issues `AL-1`, `AL-2`, and so on, independently of team `TOK`. Team keys, Project names, and labels are unique within a workspace, so two workspaces can each have team `AL` and issue `AL-1`. A new workspace has no teams; create one with + next to Teams in the sidebar, the `create_team` MCP tool, or `POST /api/w/<slug>/teams` before creating issues.

New databases use the current schema directly. Existing databases use the append-only migration mechanism. Migration 1 introduces teams: each identifier prefix already in the database becomes a team, and its numbering continues after its highest number. Migration 2 makes tables reference teams, Projects, and issues by internal id instead of team key, Project name, and identifier; the API is unchanged. After migration 2, an older binary still starts, but its issue requests fail; to go back, stop the server and restore `backups/before-migration-2.db`, losing later writes. Migration 3 introduces workspaces and moves all existing data into workspace `default` (named Default). After migration 3, an older binary still starts and reads, but creating issues, teams, Projects, or labels fails; to go back, stop the server and restore `backups/before-migration-3.db`, losing later writes. Migration 4 adds `backlog` by rebuilding issues while preserving all fields, ids, references, and the autoincrement sequence. A version 3 database is backed up to `backups/before-migration-4.db` before upgrading. Older services and pages do not support backlog; stop the server and restore the pre-upgrade backup to roll back, losing later writes. Arbitrary legacy schemas are not automatically converted. Do not delete the database to apply a schema change. See [data reliability](docs/rules/RELIABILITY.md).

Use the sidebar to enter All issues, All projects, or a Team’s Issues/Projects. Team project lists use explicit associations, including empty projects; opening a project shows all its teams. Project links use `/<slug>/project/<id>`, Team destinations use `/<slug>/team/<key>/issues` and `/<slug>/team/<key>/projects`, and the workspace project list is `/<slug>/projects`. These scopes survive refresh and browser history. Entering a destination clears temporary filters and selection, including clicking the current destination again; list/board and sorting stay within the workspace session. On narrow screens, expand Navigation to reach the same destinations. Project lists show active projects by default and offer an Archived projects view; a direct archived project link still shows its existing issues.

Use the content status selector, the search box (press Enter) to match titles and descriptions, and the type menu to filter by type; active filters appear as chips above the list, and their × clears them. Click list group headers to collapse or expand them; when the list shows every status without a search, Done and Canceled start collapsed. Click an issue to open its page at `/<slug>/issue/<identifier>`: the description and comments render as Markdown, relations and sub-issues link to each other, linked pull requests show their state and open on GitHub, the properties panel shows Team and a stable Project link, and Edit changes title, description, status, priority, and Project while keeping Team read-only. In list and board view, click an issue's status icon to change its status; in board view you can also drag a card by its grip. New issue opens a dialog with explicit Team and optional Project choices. Team issue pages preselect that Team. Project pages preselect the Project and only preselect a Team when it has exactly one associated Team; workspace and shared-project creation require a Team choice. New issues use the selected status filter, or Triage under All issues.

Issue details default to **Comments** for human discussion. **Agent work logs** lists expandable summaries in pages of 20. Settled choices also appear in **Decisions** below the description; expanding one reads its rationale. Switching activity tabs preserves the comment draft. Historical comments stay as they were because their source cannot be reliably inferred. These categories describe content, not separate permissions or verified agent identities.

Project choices are active projects associated with the selected Team. Changing Team keeps the draft and any previous Project selection; an incompatible selection must be cleared or replaced before submitting. Existing issues can change or clear Project without changing Team or identifier. Their current archived Project may be retained, but other archived projects cannot be newly assigned. Creation and editing send stable `project_id` values, with the same Service checks as REST/MCP. Failed saves retain the form; while saving, fields and closing are disabled. Successful saves show a View issue link and reload the current scope instead of inserting issues that do not match its filters. If the edit form cannot load projects, Retry projects preserves the draft.

The page lists issues with `GET /api/w/<slug>/issues`, which accepts the page's filters and returns every matching issue as the same summary MCP `list_issues` returns, without `description` and `created_at`. An issue's page reads the full issue from `GET /api/w/<slug>/issues/<identifier>`.

Use **Labels** in the sidebar to create, edit, archive, restore, and merge the workspace's issue labels. Ungrouped labels are multi-select; each group allows one value per issue. Use the **+** beside an issue's labels (**Add label** when it has none), or select issues in the list/board and choose **Change labels** for an atomic batch operation. The Labels filter supports any, all, none of the selected labels/groups, and issues with no labels. Archived labels remain on existing issues and in filters. Labels are independent of type, status, and Project.

For frontend hot reload, keep the Go server running and start:

```sh
npm --prefix web run dev
```

The Vite server proxies `/api` and `/auth` to the Go server.

## Data and backups

The server keeps one copy of the database per day in a `backups` directory next to the database file (`daily-YYYY-MM-DD.db`, newest 14 kept) and checks hourly whether today's copy exists. Before upgrading the schema it also writes `before-migration-<n>.db`, which is never pruned.

To restore, stop the server, delete the database's `-wal` and `-shm` files, copy a backup over the database file, and start the server again. Do not copy the live database file with `cp` while the server runs; most recent writes may still be in the `-wal` file.

## Import from Linear

`scripts/linear/export.py` takes a read-only snapshot of one Linear team through the Linear GraphQL API, using only the Python standard library. It reads a Linear personal API key from `~/.config/linear/api-key` (the key alone on one line; keep the file outside any repository and readable only by you) and writes JSON files into the output directory, which it creates:

```sh
python3 -I scripts/linear/export.py <output directory> <team key>
```

The output is `issues.json` (every issue of the team, including archived and trashed ones, with labels, relations, attachments, and comments), `projects.json`, `labels.json`, `states.json`, `users.json`, `documents.json`, and `team.json`; the script then prints counts. The snapshot holds the team's data: keep it out of the repository.

`inftask import-linear` copies a snapshot's issues into an existing team of a workspace, keeping their identifiers and numbers. Create the team first, with `start_number` above the highest number in the snapshot so local numbering continues after it. The command opens the database as the server does, applying any pending migrations, so stop the server that uses the database before running it:

1. Stop the server, such as with `launchctl bootout` for a LaunchAgent.
2. Back up the database: `sqlite3 <database> ".backup '<backup file>'"`.
3. Import: `inftask import-linear -data <database> -workspace <slug> -team <key> -from <snapshot directory>`. It prints what it wrote.
4. Start the server again.

The import reads `issues.json` and `projects.json` and writes in one transaction: everything, or nothing when any check fails. Before writing, it checks that the team exists, that every identifier is `<team key>-<number>`, and that every number is free in the team and below its `next_number`; the team's `next_number` does not change. Issues in the trash are skipped. Title, description, priority, and `createdAt`/`updatedAt` are kept. Statuses Triage, Backlog, Todo, Agent Ready, In Progress, In Review, Done, and Canceled map to the status of the same name; any other status stops the import. The Spec label gives type `spec`; otherwise one of Feature, Bug, Improvement, or Chore gives that type, no label gives no type, and any other label or two of those four stops the import. Every Project in `projects.json` is created unless the workspace has one with that name, which is used as it is; a new Project's description is Linear's description and then, after a blank line, its content, and it is archived when its Linear state is completed or canceled. Parents and `blocks` and `related` relations link issues by identifier; other relation types stop the import. Comments, replies included, are added in time order with their author: the user's name, or the bot's name for a bot. Attachments other than GitHub ones become links; GitHub sync records pull requests again. Each issue gets one `created` event at its `createdAt`. Estimates, cycles, milestones, assignees, creators, documents, and uploaded files are not imported.

## GitHub pull requests

The server can link GitHub pull requests to issues. It polls each repository once a minute; cloud deployments can also trigger immediate refreshes through a signed webhook:

```sh
GITHUB_TOKEN=$(gh auth token) inftask -github-repo default=owner/name -github-repo acme=owner/other
```

- `-github-repo <slug>=<owner>/<name>` turns on sync for a repository and binds it to the workspace with that slug; it can be repeated, and without it nothing syncs. A value without `<slug>=` stops the server at startup with the expected format. `GITHUB_TOKEN` must allow reading the repository's pull requests. When `-github-repo` is given without `GITHUB_TOKEN`, the server exits at startup. A workspace that does not exist yet is logged as a failed sync each round until it is created.
- A pull request links to every issue whose identifier (`<team key>-<n>` for any team of the repository's workspace, in any case) appears in its branch name or title, such as branch `al-2-github-sync` or title `AL-2 Sync pull requests`. Teams of other workspaces are not matched, even with the same key. The teams are read again on every sync, so a new team's identifiers are recognized from the next sync on; pull requests read before the team existed are not read again. The pull request description is ignored because it often mentions other issues. A pull request may name an issue of an existing team that does not exist yet; it shows on the issue once the issue is created.
- `get_issue` and `GET /api/w/<slug>/issues/<identifier>` list linked pull requests under `pull_requests` with repo, number, title, url, state (`open`, `draft`, `merged`, or `closed`), and `updated_at`.
- Status only moves forward. An open, non-draft pull request moves an issue in `triage`, `backlog`, `todo`, `agent_ready`, or `in_progress` to `in_review`. A merged pull request moves an issue that is not `done` or `canceled` to `done`. Draft pull requests, pull requests closed without merging, and pull requests reopened after their issue is done change nothing. Each change adds a `status_changed` event whose detail ends with `via owner/name#<number>`.
- Each sync reads pull requests by most recent update and stops at the first one updated before the previous sync; the first sync reads them all. A repository that fails is logged and read again in the next round.

`-github-linkback` (off by default) comments on an open or draft pull request the first time it is linked to an existing issue: the identifier and title in bold, then the issue description in a collapsed `<details>` block. **This copies the issue's title and description into GitHub, where everyone who can see the repository can read them; on a public repository that is everyone.** Merged and closed pull requests get no comment, so the first sync does not comment on old pull requests. Linkback needs a token that can comment on pull requests.

Set `GITHUB_WEBHOOK_SECRET` to enable `POST /webhooks/github`; at least one `-github-repo` is required. Configure GitHub to send `pull_request` events as JSON to that path over HTTPS, with the same webhook secret. Valid deliveries for configured repositories schedule a serial refresh through the existing sync worker. Pending notifications coalesce; the payload itself never writes PR state. Polling recovers downtime and failed reads. A `202` means scheduled, not that the ticket has already changed.

## Cloud team instance

The cloud instance uses InfOne UC for sign-in. All authenticated UC users see every workspace and can use all existing operations. There are no workspace members, roles, or tenant isolation. The UC client may temporarily be the existing HarnessKit client; configure it on the backend rather than copying credentials into this repository.

See [cloud deployment and recovery](docs/DEVELOPMENT.md#云端团队部署) for configuration, Compose, and the remote MCP OAuth contract. Deployment artifacts are [Dockerfile](Dockerfile), [Compose](deploy/compose.yaml), and [Caddyfile](deploy/Caddyfile). The single instance keeps SQLite on a persistent volume; it is not a multi-replica deployment.

## macOS desktop client

The [Electron client](desktop/README.md) connects to the existing cloud instance. It does not bundle a Go server or a local database, and does not change your agent's cloud MCP configuration. On macOS:

```sh
npm --prefix desktop ci
npm --prefix desktop start
# Build an Apple Silicon .app and .dmg for local testing:
npm --prefix desktop run dist:mac
```

The DMG is at `desktop/dist/InfTask-0.1.0-arm64.dmg`. Sign in through UC inside the app. Closing its window keeps the app in the Dock; Quit exits it. The default build has an ad-hoc signature for local use, not a Developer ID signature or Apple notarization. See [desktop build, session, security, and distribution notes](desktop/README.md).

## Human assignment

Issues have one human assignee or are unassigned. Any active member who can edit an issue may reassign or clear it. Public teams accept any active workspace member; private teams accept their active team members. Suspended users remain visible on past assignments but cannot receive new ones. Assignment does not lock editing or represent an agent session.

Choose an assignee during creation or in issue details, and filter by yourself, unassigned, or a workspace member. Assignment history shows who changed responsibility and its before/after values. REST/MCP create and update accept `assignee` as a user subject; empty string clears and omitted update leaves unchanged. Reads return the person profile or null. `list_assignees(team)` and `GET /api/w/<slug>/teams/<key>/assignees` list eligible people. Lists accept `assignee` (empty means unassigned) or `assigned_to_me=true`, but not both.

## MCP

The running server exposes these tools over streamable HTTP at `/w/<slug>/mcp`. Each request reads and writes the workspace in its path, and no tool takes a workspace argument; there are no tools to manage workspaces. A slug that does not exist returns 404.

- `list_issues`: filter by status, priority, type, team key, Project name, parent identifier, title/description query, and labels (AND). `label_ids` and `label_group_ids` select predicates combined using `label_mode`: `any` (default), `all`, or `none`; `unlabeled` requires empty selections. Returns summaries including labels and priority, with a default limit of 50. Accepts `sort=priority|updated`; defaults to priority for `status=agent_ready`, otherwise `updated_at` descending.
- `get_issue`: retrieve the issue, decision summaries and `work_log_count` (record bodies are read separately), comments (each with an `author`, empty for comments added here), links, linked GitHub pull requests, parent identifier, children summaries, and `blocks`, `blocked_by`, and `related` summaries.
- `create_issue`: create an issue with a required team key and title; optionally set description, status, type, Project, and parent. There is no default team. The issue gets the team's next number in one transaction with the insert.
- `list_teams`: list teams with their key, name, `next_number`, and `emoji`.
- `create_team`: create a team with a key of 2 to 6 uppercase letters or digits starting with a letter, and a required name. Numbering starts at 1, or at the optional `start_number`, so a team imported from another tracker does not reuse its old numbers. Optional `emoji` sets its Unicode emoji icon. A key cannot be changed, and teams cannot be deleted.
- `update_team`: update supplied display `name` and/or `emoji` by immutable team key; omission preserves either field and an empty emoji restores the default.
- `update_issue`: update supplied fields only; an empty string clears type, Project, or parent.
- `add_comment`: append a human discussion comment; agents use this only when the user explicitly asks them to post a comment. Comments cannot be edited or deleted.
- `add_work_log`: append an agent record with a required single-line `summary` (up to 240 characters) and Markdown `body` (up to 65,536 bytes). `kind` is `progress` (default) or `decision`; decisions explain a settled choice and its reasons. Records cannot be edited or deleted.
- `list_work_logs`: retrieve summaries newest first, optionally filtered by `kind`; `limit` defaults to 20 (maximum 100), and `before` follows the returned `next_before` cursor (0 means no more records).
- `get_work_log`: read one record body by issue `identifier` and record `id`.
- `add_relation`, `remove_relation`: manage directed `blocks` (or its inverse `blocked_by`) and undirected `related` relations. Duplicate additions succeed without creating duplicates; self-relations are forbidden.
- `add_link`: attach a title and URL to an issue.
- `list_labels`, `list_label_groups`: discover existing labels and single-select groups. Labels exclude archived entries unless `include_archived` is true.
- `create_label`, `update_label`: create labels or edit supplied name/color/description/archived properties by stable ID. Names are trimmed and case-insensitively unique within a group; colors use `#RRGGBB`. `group_id: 0` means ungrouped; membership is fixed after creation.
- `save_label_group`: create a group, or rename an existing group by `id`; group names are unique in the workspace and groups cannot nest.
- `change_issue_labels`: supply `identifiers` and incremental `add`/`remove` label ID arrays. The whole batch succeeds or rolls back. Adding a grouped label replaces the old selection; duplicate additions/removals are idempotent. Multiple additions in one group and overlapping add/remove arrays are rejected. Archived labels can be removed or retained, but cannot be newly applied.
- `merge_labels`: transfer `source_id` associations to an active `target_id` in the same group, then delete the source. External filters referencing the source ID must be updated.
- `list_projects`: list unarchived Projects by default; set `include_archived` to include all and `team` to filter by explicit association, including empty Projects.
- `create_project`: supply `name`, optional `description`, and a required nonempty `teams` array of existing Team keys, for example `{ "name": "Release", "teams": ["AL"] }`.
- `get_project`, `update_project`: address by stable `id` or current `name`; both must agree when supplied together. Update only supplied `new_name`, `teams`, description, archived state, or emoji. Omitted teams stay unchanged; an empty array is rejected. `new_name` renames the Project without changing its stable id or issue references; current `name` remains a lookup reference. Names are trimmed, nonempty, and unique within the workspace; a duplicate returns a conflict without partial updates. Old names are not aliases.

Projects can be created from All projects or a Team’s Projects page. The latter preselects that Team; workspace creation requires an explicit Team selection. Edit project changes name, description, and associated Teams without changing its URL. Historical unassigned Projects can receive Teams or have only their metadata edited. Archive project removes it from active lists; Archived projects links to its stable page for viewing and restoring. Failed saves preserve the draft, and cancel makes no request.

Team display names can be changed with the pencil button beside a Team in the sidebar or its selected heading. The dialog preserves input on failure; cancel makes no request. Successful saves update the sidebar and create-issue Team choices. `update_team` and REST `PATCH /api/w/<slug>/teams/<key>` accept `name`; omitted name stays unchanged, and whitespace-only names are rejected. Team keys, internal ids, numbering, and existing issue identifiers stay fixed.

Project and team icons use a single fully-qualified Unicode 16.0 emoji, including flags, skin tones, and joined sequences. Empty `emoji` restores the Project box or team initials; other strings are rejected. Click a Team icon in the sidebar or its current heading, or the icon in a Project page heading, to open the searchable, grouped emoji picker. Project list names open their stable pages; sidebar destinations enter scopes rather than applying hidden filters. Create team also offers an emoji draft. The catalog is bundled locally, rendered with system emoji fonts, and the browser's recent choices appear with common defaults under Frequently used. REST `PATCH /api/w/<slug>/projects/<encoded name>` and `/teams/<key>` accept `{ "emoji": "🚀" }`; the team route edits display `name` and emoji, keeping key and numbering unchanged. The Project route also edits `new_name`, description, archived state, and teams. Project detail and editing are available at `GET/PATCH /api/w/<slug>/projects/by-id/<id>` and the current-name route; `POST /api/w/<slug>/projects` creates a Project. The Project list accepts `team` and `include_archived=true`.

Project responses contain stable `id` and associated `teams` (Team keys). Issues return `project_id` alongside the current Project name; create, update, and list accept `project_id` or `project`, with matching references required when both are supplied. Omitted update fields stay unchanged; `project_id: 0` or `project: ""` clears the Project. A Project only accepts issues from associated Teams. Removing an association with any existing issue, including closed issues, is rejected atomically. Archived Projects retain existing issues but reject new assignments. Historical empty Projects remain unassigned, appear only in workspace lists, and must receive teams before accepting issues. Linear import maintains associations in its original transaction and preserves historical archived assignments.

Priority follows Linear: `0` No priority (default), `1` Urgent, `2` High, `3` Medium, `4` Low. Create/update accept `priority`; omission on update preserves it and explicit `0` clears it. List filters accept `priority` and `sort=priority|updated`: the default is priority for `status=agent_ready`, otherwise newest updated first. Priority order places No priority last, with ties broken by newest update and then newest internal id. The UI provides priority controls in the list, board, details, and create/edit dialogs, plus filtering and ordering. Its default ordering applies priority within the Agent ready group and last updated within other status groups.

Statuses are `triage` (new and undecided; the default), `backlog` (kept but not scheduled), `todo` (committed work), `agent_ready`, `in_progress`, `in_review`, `done`, and `canceled`. Optional types are `spec`, `feature`, `bug`, `improvement`, and `chore`. Team, Project, parent, relation, and label references must already exist in the same workspace. An issue cannot be its own parent; parent cycles are not checked. Blocking dependencies (`blocks` / `blocked_by`) form a DAG: the Service rejects cycles with `dependency cycle is not allowed`, and SQLite triggers enforce this on INSERT and UPDATE, including direct SQL and Linear imports. Repeated additions stay idempotent. `related` and parent links are not dependency edges. Existing databases are checked during migration; a legacy dependency cycle stops the upgrade without deleting relations. See [data reliability](docs/rules/RELIABILITY.md) for recovery.

Missing issues, teams, or Projects, including those of another workspace, return not-found errors. SQLite constraints enforce unique team keys, Project names, and label names within a workspace, valid stored values, foreign keys, and the prohibition on self-parenting and self-relations. Only issue creation and status changes add events.

It is the only process that opens the database, so the page and every agent see the same data. Keep the server running while agents work:

```sh
codex mcp add inftask --url http://127.0.0.1:8787/w/default/mcp
claude mcp add --transport http inftask http://127.0.0.1:8787/w/default/mcp
```

The server binds to loopback by default and wraps requests in Go cross-origin protection. Local mode has no separate `Host` allowlist or user authentication. Non-loopback listeners require UC authentication; see [security boundaries](docs/rules/SECURITY.md).

## Agent plugin

This repository is also a plugin marketplace for Claude Code and Codex. Its `inftask` plugin adds:

- the MCP server `inftask` at the maintainer's cloud instance, `https://harnessagent.tokensinfinity.com/w/default/mcp`, which signs in with the public OAuth client `inftask`;
- a `ticket` skill for finding, creating, and updating tickets from any repository. It takes the team key and workspace from the repository's `AGENTS.md` and asks when they are missing.

The plugin has no hooks. The marketplaces are [.claude-plugin/marketplace.json](.claude-plugin/marketplace.json) for Claude Code and [.agents/plugins/marketplace.json](.agents/plugins/marketplace.json) for Codex; the plugin is in [plugins/inftask/](plugins/inftask/). This repository's own [.agents/skills/](.agents/skills/) are its contribution workflow, not part of the plugin.

Claude Code:

```sh
claude plugin marketplace add DengNaichen/inftask
claude plugin install inftask@inftask
```

Codex:

```sh
codex plugin marketplace add DengNaichen/inftask
codex plugin add inftask@inftask
```

A path to a local checkout works in place of `DengNaichen/inftask`. Start a new session after installing.

The cloud MCP accepts only UC access tokens, so each client signs in once in the browser before its first tool call. In Claude Code, run `/mcp`, select `plugin:inftask:inftask`, and authenticate. In Codex, run `codex mcp login inftask`; `--no-browser` prints the authorization URL instead of opening a browser.

To use another instance or workspace, override the server in your client's own configuration; the plugin's address is fixed. Every InfTask server uses the public client ID `inftask`, so only the URL changes. A local instance without sign-in needs no client ID.

- Codex: a server named `inftask` in your Codex configuration replaces the plugin's. Run `codex mcp add inftask --url https://<host>/w/<slug>/mcp --oauth-client-id inftask`, then `codex mcp login inftask`. `codex mcp remove inftask` goes back to the plugin's server.
- Claude Code: a server with a different URL does not replace the plugin's. Add your own with `claude mcp add --transport http --scope user --client-id inftask inftask https://<host>/w/<slug>/mcp`, authenticate it in `/mcp`, and turn off `plugin:inftask:inftask` there. Claude Code records that choice per project.

## Run as a macOS LaunchAgent

launchd can keep a long-lived instance running: it starts the server at login and restarts it when it exits. Give that instance a non-default port, such as 8700, and keep the default 8787 for development builds, so a development server never opens the instance's database. Register MCP clients with the instance's port and the repository's workspace, such as `http://127.0.0.1:8700/w/default/mcp`.

Save the following as `~/Library/LaunchAgents/com.example.inftask.plist`, replacing `/Users/you`, `default=owner/name`, and the label with your own. launchd does not expand `~` or `$HOME` and starts jobs with a minimal `PATH`, so every path is absolute, including `gh` (`which gh` prints it) and the `inftask` binary.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.example.inftask</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/sh</string>
    <string>-c</string>
    <string>GITHUB_TOKEN="$(/opt/homebrew/bin/gh auth token)" exec /Users/you/.local/bin/inftask -addr 127.0.0.1:8700 -data "/Users/you/Library/Application Support/inftask/inftask.db" -github-repo default=owner/name</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/Users/you/Library/Logs/inftask.log</string>
  <key>StandardErrorPath</key>
  <string>/Users/you/Library/Logs/inftask.log</string>
</dict>
</plist>
```

The token is not written into the plist: `/bin/sh -c` asks `gh auth token` for it each time the job starts, and `exec` then replaces the shell with the server. Without GitHub sync, drop `GITHUB_TOKEN="$(…)"` and `-github-repo`. The server does not create the database's directory, so create it once, then load the job:

```sh
mkdir -p ~/Library/"Application Support"/inftask
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.example.inftask.plist
```

After rebuilding, install the binary at the path the plist names and restart the job:

```sh
make build
install -m 755 inftask /Users/you/.local/bin/inftask
launchctl kickstart -k gui/$(id -u)/com.example.inftask
```

To stop the job and unload it:

```sh
launchctl bootout gui/$(id -u)/com.example.inftask
```

`kickstart` does not reread the plist. After editing the plist, run `bootout` and then `bootstrap` again. A `bootstrap` right after `bootout` can fail with `Bootstrap failed: 5: Input/output error` while the old job is still shutting down; wait a few seconds and retry. Startup errors, such as a missing `GITHUB_TOKEN`, go to `~/Library/Logs/inftask.log`.

## Validate

```sh
make test
```


## License

[Apache-2.0](LICENSE)

## Cycles

Teams can enable repeating 1–8 week cycles under **Cycles → Cycle settings**, choosing an IANA timezone, first day and 1–15 upcoming cycles. An issue has at most one cycle from its own team, independent of its Project and status. Create/edit issues with `cycle_id` (0 clears); list with `cycle_id` (0 means unassigned). MCP tools are `list_cycles`, `configure_cycles`, `get_cycle`, and `update_cycle`.

At each boundary, committed unfinished work (`todo`, `agent_ready`, `in_progress`, `in_review`) rolls into the next cycle. Other statuses stay in the closing cycle. Completion counts are saved snapshots; the issue list shows current assignments. Startup and periodic maintenance catch up missed boundaries atomically. Disabling closes the current cycle, removes upcoming plans and clears their assignments while preserving completed history. Disable before changing cadence; a new schedule must not overlap history. Capacity forecasting, cooldowns, calendar feeds and Linear cycle import are not included.
