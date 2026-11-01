package mcpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type identifierInput struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier <team key>-<number>, such as AL-1"`
}

type listOutput struct {
	Issues []app.IssueSummary `json:"issues" jsonschema:"issue summaries without descriptions, including priority and ordered by the requested sort"`
}

type commentInput struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier, such as AL-1"`
	Body       string `json:"body" jsonschema:"comment body; comments are append-only"`
}

type relationInput struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier, such as AL-1"`
	Kind       string `json:"kind" jsonschema:"relation kind: blocks (this issue blocks other), blocked_by (other blocks this issue), or related (undirected)"`
	Other      string `json:"other" jsonschema:"other issue identifier; must differ from identifier"`
}

type linkInput struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier, such as AL-1"`
	Title      string `json:"title" jsonschema:"link title"`
	URL        string `json:"url" jsonschema:"link URL"`
}

type listProjectsInput struct {
	Team            string `json:"team,omitempty" jsonschema:"filter by associated Team key; empty lists the workspace"`
	IncludeArchived bool   `json:"include_archived,omitempty" jsonschema:"include archived Projects; defaults to false"`
}

type projectsOutput struct {
	Projects []app.Project `json:"projects"`
}

type teamsOutput struct {
	Teams []app.Team `json:"teams"`
}

type createProjectInput struct {
	Teams       []string `json:"teams" jsonschema:"required nonempty list of existing Team keys in this workspace"`
	Name        string   `json:"name" jsonschema:"unique Project name"`
	Description string   `json:"description,omitempty" jsonschema:"the outcome this Project should deliver"`
}

// Handler serves the tools over streamable HTTP at /w/{workspace}/mcp. Each
// request reads and writes the workspace in its path, and is self-contained,
// so clients survive server restarts without re-initializing. An unknown
// workspace is not found.
func Handler(service *app.Service) http.Handler {
	options := &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workspace, err := service.InWorkspace(r.Context(), r.PathValue("workspace"))
		if errors.Is(err, app.ErrWorkspaceNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if app.KindOf(err) == app.KindForbidden {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		server := New(workspace)
		mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, options).ServeHTTP(w, r)
	})
}

func New(service *app.Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "inftask", Version: "0.1.0"}, nil)
	cycleTools(server, service)
	addInitiativeTools(server, service)
	addSpecTools(server, service)
	statusNames := make([]string, len(app.Statuses))
	for i, status := range app.Statuses {
		statusNames[i] = string(status)
	}
	statusList := strings.Join(statusNames, ", ")
	typeList := strings.Join(app.Types, ", ")
	statuses := "Statuses: " + statusList + ". "
	types := "Types: " + typeList + "; empty string means no type. "
	addTool(server, issueTool[app.ListIssues](&mcp.Tool{Name: "list_issues", Description: "List summaries without descriptions, including priority. Default sort: priority for status=agent_ready, otherwise newest updated first. Explicit sort=priority or updated overrides; no priority sorts last; ties use newest updated then newest id. All filters combine with AND; team filters by team key; limit defaults to 50. " + statuses + types}, statusList, typeList),
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.ListIssues) (*mcp.CallToolResult, listOutput, error) {
			issues, err := service.List(ctx, input)
			return nil, listOutput{Issues: app.Summaries(issues)}, err
		})
	addTool(server, &mcp.Tool{Name: "get_issue", Description: "Get an issue with human comments, decision summaries and work_log_count (use list_work_logs and get_work_log for agent records), links, linked GitHub pull requests, parent identifier, children summaries, and blocks, blocked_by, related summaries. " + statuses + types},
		func(ctx context.Context, _ *mcp.CallToolRequest, input identifierInput) (*mcp.CallToolResult, app.IssueDetails, error) {
			issue, err := service.Details(ctx, input.Identifier)
			return nil, issue, err
		})
	addTool(server, issueTool[app.CreateIssue](&mcp.Tool{Name: "create_issue", Description: "Create an issue in a team; team and title are required, status defaults to triage. There is no default team: use the team key the repository names, or list_teams. The identifier is <team key>-<n>, where n is the team's next number. Project and parent must already exist. " + statuses + types}, statusList, typeList),
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.CreateIssue) (*mcp.CallToolResult, app.Issue, error) {
			issue, err := service.Create(ctx, input)
			return nil, issue, err
		})
	addTool(server, issueTool[app.UpdateIssue](&mcp.Tool{Name: "update_issue", Description: "Update only supplied fields; omitted priority is unchanged and priority=0 clears it. Empty strings clear type, project, or parent. Parent cannot be self. " + statuses + types}, statusList, typeList),
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.UpdateIssue) (*mcp.CallToolResult, app.Issue, error) {
			issue, err := service.Update(ctx, input)
			return nil, issue, err
		})
	addTool(server, &mcp.Tool{Name: "add_comment", Description: "Append a human discussion comment, only when the user explicitly asks you to post one. Use add_work_log for agent progress, findings, verification and decisions. Comments cannot be edited or deleted."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input commentInput) (*mcp.CallToolResult, app.Comment, error) {
			comment, err := service.AddComment(ctx, input.Identifier, input.Body)
			return nil, comment, err
		})
	addTool(server, &mcp.Tool{Name: "add_work_log", Description: "Append an agent work record, separate from human comments. Use progress for findings, verification and handoff; decision for a settled choice and its reasons. Supply a concise summary and Markdown body. Append-only; classification does not establish agent identity or confer permissions."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.AddWorkLog) (*mcp.CallToolResult, app.WorkLog, error) {
			log, err := service.AddWorkLog(ctx, input)
			return nil, log, err
		})
	addTool(server, &mcp.Tool{Name: "list_work_logs", Description: "List agent work record summaries, newest first, with optional kind filter. Follow next_before for older pages; zero means the end. Use get_work_log to read a body."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.ListWorkLogs) (*mcp.CallToolResult, app.WorkLogPage, error) {
			page, err := service.ListWorkLogs(ctx, input)
			return nil, page, err
		})
	addTool(server, &mcp.Tool{Name: "get_work_log", Description: "Read one agent work record including its Markdown body. Both issue identifier and record id must match and be accessible."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.GetWorkLog) (*mcp.CallToolResult, app.WorkLog, error) {
			log, err := service.GetWorkLog(ctx, input)
			return nil, log, err
		})
	for _, operation := range []struct {
		name, description string
		remove            bool
	}{
		{"add_relation", "Add a relation; duplicate additions succeed without changing it.", false},
		{"remove_relation", "Remove a relation; removing an absent relation succeeds.", true},
	} {
		addTool(server, &mcp.Tool{Name: operation.name, Description: operation.description + " Kinds: blocks (this issue blocks other), blocked_by (other blocks this issue), related (undirected). Self-relations are forbidden. Blocking dependencies must be acyclic; additions that create a cycle are rejected."},
			func(ctx context.Context, _ *mcp.CallToolRequest, input relationInput) (*mcp.CallToolResult, struct{}, error) {
				err := service.Relation(ctx, input.Identifier, input.Kind, input.Other, operation.remove)
				return nil, struct{}{}, err
			})
	}
	addTool(server, &mcp.Tool{Name: "add_link", Description: "Attach a titled URL to an issue."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input linkInput) (*mcp.CallToolResult, app.Link, error) {
			link, err := service.AddLink(ctx, input.Identifier, input.Title, input.URL)
			return nil, link, err
		})
	addTool(server, &mcp.Tool{Name: "list_teams", Description: "List teams by key. Each team numbers its own issues <key>-<n>; next_number is the number its next issue gets."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, teamsOutput, error) {
			teams, err := service.ListTeams(ctx)
			return nil, teamsOutput{Teams: teams}, err
		})
	addTool(server, &mcp.Tool{Name: "create_team", Description: "Create a team whose issues are numbered <key>-1, <key>-2, and so on. key is 2 to 6 uppercase letters or digits starting with a letter; name is required. start_number makes numbering begin above numbers the team already used elsewhere, such as a team imported from another tracker. Optional emoji chooses a Unicode emoji icon. The creator joins as owner. Optional private restricts content to team members. A key cannot be changed; team deletion is unavailable."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.CreateTeam) (*mcp.CallToolResult, app.Team, error) {
			team, err := service.CreateTeam(ctx, input)
			return nil, team, err
		})
	addTool(server, &mcp.Tool{Name: "update_team", Description: "Update a team display name or emoji by key; omitted fields are unchanged and empty emoji clears it. Name must be nonempty. Only owners or administrators can change private visibility. Key and numbering cannot be changed."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.UpdateTeam) (*mcp.CallToolResult, app.Team, error) {
			team, err := service.UpdateTeam(ctx, input)
			return nil, team, err
		})
	addTool(server, &mcp.Tool{Name: "list_projects", Description: "List Projects alphabetically; archived Projects are excluded unless include_archived is true."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input listProjectsInput) (*mcp.CallToolResult, projectsOutput, error) {
			projects, err := service.ListProjects(ctx, input.IncludeArchived, input.Team)
			return nil, projectsOutput{Projects: projects}, err
		})
	addTool(server, &mcp.Tool{Name: "create_project", Description: "Create a Project with a unique name, at least one Team key in teams, and an optional outcome description. New Projects are unarchived."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input createProjectInput) (*mcp.CallToolResult, app.Project, error) {
			project, err := service.CreateProject(ctx, input.Name, input.Description, input.Teams...)
			return nil, project, err
		})
	addTool(server, &mcp.Tool{Name: "get_project", Description: "Get a Project by stable id or current name; both must match when supplied."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.ProjectRef) (*mcp.CallToolResult, app.Project, error) {
			p, err := service.GetProject(ctx, input)
			return nil, p, err
		})
	addTool(server, &mcp.Tool{Name: "update_project", Description: "Find a Project by stable id or current name and update only supplied new_name, teams, description, archived, or emoji fields. Empty emoji restores the default icon. Current name remains a lookup reference; new_name renames without changing the stable id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.UpdateProject) (*mcp.CallToolResult, app.Project, error) {
			project, err := service.UpdateProject(ctx, input)
			return nil, project, err
		})
	addLabelTools(server, service)
	addTeamAccessTools(server, service)
	addAssigneeTools(server, service)
	addTool(server, &mcp.Tool{Name: "delegate_issue", Description: "Register the calling coding agent session as working on an issue. The issue must have a human assignee before every call, including repeats; assign one first if missing. Main agents and subagents can both delegate themselves. Supply only identifier; the repository PreToolUse hook injects client_context. Context is required by the Service; never invent it. Repeated calls from the same agent/session/agent_id/hostname return the original record. Every successful call, including repeats, sets status to in_progress; other statuses (including in_review, done and canceled) restart work. Already in_progress leaves updated_at and status history unchanged. Registration and status changes are atomic. Does not change human assignee, start a process, or grant exclusive access."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.DelegateIssue) (*mcp.CallToolResult, app.Delegation, error) {
			delegation, err := service.Delegate(ctx, input)
			return nil, delegation, err
		})
	return server
}

// issueTool infers the input schema of In and appends the allowed values to
// the description of its status and type properties. The values are
// documentation only; the Service validates them.
func issueTool[In any](tool *mcp.Tool, statusList, typeList string) *mcp.Tool {
	schema, err := jsonschema.For[In](nil)
	if err != nil {
		panic(err)
	}
	schema.Properties["status"].Description += ". Statuses: " + statusList
	schema.Properties["type"].Description += ". Types: " + typeList
	tool.InputSchema = schema
	return tool
}
