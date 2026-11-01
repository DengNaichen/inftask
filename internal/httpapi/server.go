package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/DengNaichen/inftask/internal/app"
)

type server struct{ service *app.Service }

// New serves the REST API, the MCP endpoint of each workspace, and the web UI.
// Browsers may only reach them from the same origin; MCP clients send no
// Origin and pass through.
func New(service *app.Service, web, mcp http.Handler) http.Handler {
	s := &server{service: service}
	mux := http.NewServeMux()
	s.cycleRoutes(mux)
	s.workLogRoutes(mux)
	s.specRoutes(mux)
	s.labelRoutes(mux)
	s.accessRoutes(mux)
	s.teamAccessRoutes(mux)
	mux.HandleFunc("GET /api/w/{workspace}/teams/{key}/assignees", s.in((*server).listAssignees))
	mux.HandleFunc("GET /auth/session", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
	})
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/workspaces", s.listWorkspaces)
	mux.HandleFunc("POST /api/workspaces", s.createWorkspace)
	mux.HandleFunc("PATCH /api/workspaces/{workspace}", s.renameWorkspace)
	mux.HandleFunc("GET /api/w/{workspace}/issues", s.in((*server).listIssues))
	mux.HandleFunc("POST /api/w/{workspace}/issues", s.in((*server).createIssue))
	mux.HandleFunc("PATCH /api/w/{workspace}/issues/{identifier}/status", s.in((*server).moveIssue))
	mux.HandleFunc("PATCH /api/w/{workspace}/issues/{identifier}", s.in((*server).updateIssue))
	mux.HandleFunc("GET /api/w/{workspace}/issues/{identifier}", s.in((*server).getIssue))
	mux.HandleFunc("POST /api/w/{workspace}/issues/{identifier}/comments", s.in((*server).addComment))
	mux.HandleFunc("GET /api/w/{workspace}/projects", s.in((*server).listProjects))
	mux.HandleFunc("GET /api/w/{workspace}/initiatives", s.in((*server).listInitiatives))
	mux.HandleFunc("POST /api/w/{workspace}/initiatives", s.in((*server).createInitiative))
	mux.HandleFunc("GET /api/w/{workspace}/initiatives/{id}", s.in((*server).getInitiative))
	mux.HandleFunc("PATCH /api/w/{workspace}/initiatives/{id}", s.in((*server).updateInitiative))
	mux.HandleFunc("POST /api/w/{workspace}/projects", s.in((*server).createProject))
	mux.HandleFunc("GET /api/w/{workspace}/projects/{name}", s.in((*server).getProject))
	mux.HandleFunc("GET /api/w/{workspace}/projects/by-id/{id}", s.in((*server).getProject))
	mux.HandleFunc("PATCH /api/w/{workspace}/projects/by-id/{id}", s.in((*server).updateProject))
	mux.HandleFunc("PATCH /api/w/{workspace}/projects/{name}", s.in((*server).updateProject))
	mux.HandleFunc("PATCH /api/w/{workspace}/teams/{key}", s.in((*server).updateTeam))
	mux.HandleFunc("GET /api/w/{workspace}/teams", s.in((*server).listTeams))
	mux.HandleFunc("POST /api/w/{workspace}/teams", s.in((*server).createTeam))
	for _, method := range []string{"GET", "POST", "DELETE"} {
		mux.Handle(method+" /w/{workspace}/mcp", mcp)
	}
	mux.Handle("GET /", web)
	return http.NewCrossOriginProtection().Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path != "/api/health" {
			if err := service.RememberIdentity(r.Context()); err != nil {
				writeError(w, err)
				return
			}
		}
		mux.ServeHTTP(w, r)
	}))
}

// in serves a route of the workspace named in the path; handle reads and
// writes only that workspace. An unknown slug is not found.
func (s *server) in(handle func(*server, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspace, err := s.service.InWorkspace(r.Context(), r.PathValue("workspace"))
		if err != nil {
			writeError(w, err)
			return
		}
		handle(&server{service: workspace}, w, r)
	}
}

func (s *server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := s.service.ListWorkspaces(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces})
}

func (s *server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var input app.Workspace
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	workspace, err := s.service.CreateWorkspace(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, workspace)
}

// renameWorkspace changes only the name; the slug cannot change.
func (s *server) renameWorkspace(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	workspace, err := s.service.RenameWorkspace(r.Context(), r.PathValue("workspace"), input.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workspace)
}

func (s *server) listIssues(w http.ResponseWriter, r *http.Request) {
	status := app.Status(r.URL.Query().Get("status"))
	issueType := r.URL.Query().Get("type")
	limit := -1
	input := app.ListIssues{Sort: r.URL.Query().Get("sort"), Team: r.URL.Query().Get("team"), Query: r.URL.Query().Get("query"), Limit: &limit}
	if r.URL.Query().Has("assignee") {
		input.Assignee = new(r.URL.Query().Get("assignee"))
	}
	if r.URL.Query().Has("assigned_to_me") {
		value, err := strconv.ParseBool(r.URL.Query().Get("assigned_to_me"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid assigned_to_me"})
			return
		}
		input.AssignedToMe = value
	}
	if r.URL.Query().Has("project") {
		input.Project = new(r.URL.Query().Get("project"))
	}
	if r.URL.Query().Has("cycle_id") {
		id, err := strconv.ParseInt(r.URL.Query().Get("cycle_id"), 10, 64)
		if err != nil {
			writeError(w, app.ErrCycleReference)
			return
		}
		input.CycleID = &id
	}
	if r.URL.Query().Has("project_id") {
		id, err := strconv.ParseInt(r.URL.Query().Get("project_id"), 10, 64)
		if err != nil {
			writeError(w, app.ErrProjectReference)
			return
		}
		input.ProjectID = &id
	}
	if r.URL.Query().Has("priority") {
		priority, err := strconv.Atoi(r.URL.Query().Get("priority"))
		if err != nil {
			writeError(w, app.ErrInvalidPriority)
			return
		}
		input.Priority = &priority
	}
	if r.URL.Query().Has("status") {
		input.Status = &status
	}
	if r.URL.Query().Has("type") {
		input.Type = &issueType
	}
	if err := labelQuery(r, &input); err != nil {
		writeError(w, err)
		return
	}
	issues, err := s.service.List(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"issues": app.Summaries(issues)})
}

func (s *server) createIssue(w http.ResponseWriter, r *http.Request) {
	var input app.CreateIssue
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	issue, err := s.service.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, issue)
}

func (s *server) moveIssue(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status app.Status `json:"status"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	issue, err := s.service.Update(r.Context(), app.UpdateIssue{Identifier: r.PathValue("identifier"), Status: &input.Status})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (s *server) updateIssue(w http.ResponseWriter, r *http.Request) {
	var input app.UpdateIssue
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	input.Identifier = r.PathValue("identifier")
	issue, err := s.service.Update(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (s *server) getIssue(w http.ResponseWriter, r *http.Request) {
	issue, err := s.service.Details(r.Context(), r.PathValue("identifier"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (s *server) addComment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	comment, err := s.service.AddComment(r.Context(), r.PathValue("identifier"), input.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

func (s *server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.service.ListProjects(r.Context(), r.URL.Query().Get("include_archived") == "true", r.URL.Query().Get("team"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (s *server) listTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := s.service.ListTeams(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

func (s *server) createTeam(w http.ResponseWriter, r *http.Request) {
	var input app.CreateTeam
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	team, err := s.service.CreateTeam(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, team)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch app.KindOf(err) {
	case app.KindNotFound:
		status = http.StatusNotFound
	case app.KindInvalid:
		status = http.StatusBadRequest
	case app.KindForbidden:
		status = http.StatusForbidden
	case app.KindConflict:
		status = http.StatusConflict
	default:
		if _, ok := err.(*json.SyntaxError); ok {
			status = http.StatusBadRequest
		}
	}
	message := err.Error()
	if status == http.StatusInternalServerError {
		message = "internal server error"
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
