package httpapi

import (
	"net/http"
	"strconv"

	"github.com/DengNaichen/inftask/internal/app"
)

func (s *server) specRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/w/{workspace}/specs", s.in((*server).listSpecs))
	mux.HandleFunc("POST /api/w/{workspace}/specs", s.in((*server).createSpec))
	mux.HandleFunc("GET /api/w/{workspace}/specs/{identifier}", s.in((*server).getSpec))
	mux.HandleFunc("PUT /api/w/{workspace}/specs/{identifier}", s.in((*server).updateSpec))
	mux.HandleFunc("GET /api/w/{workspace}/specs/{identifier}/revisions", s.in((*server).listSpecRevisions))
	mux.HandleFunc("GET /api/w/{workspace}/specs/{identifier}/revisions/{number}", s.in((*server).getSpecRevision))
}

func (s *server) listSpecs(w http.ResponseWriter, r *http.Request) {
	for key := range r.URL.Query() {
		if key != "team" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown query parameter " + key})
			return
		}
	}
	specs, err := s.service.ListSpecs(r.Context(), r.URL.Query().Get("team"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"specs": specs})
}

func (s *server) createSpec(w http.ResponseWriter, r *http.Request) {
	var input app.CreateSpec
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	spec, err := s.service.CreateSpec(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, spec)
}

func (s *server) getSpec(w http.ResponseWriter, r *http.Request) {
	spec, err := s.service.GetSpec(r.Context(), r.PathValue("identifier"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, spec)
}

func (s *server) updateSpec(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BaseRevision int             `json:"base_revision"`
		Content      app.SpecContent `json:"content"`
		Message      string          `json:"message"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	spec, err := s.service.UpdateSpec(r.Context(), app.UpdateSpec{Identifier: r.PathValue("identifier"), BaseRevision: body.BaseRevision, Content: body.Content, Message: body.Message})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, spec)
}

func (s *server) listSpecRevisions(w http.ResponseWriter, r *http.Request) {
	revisions, err := s.service.ListSpecRevisions(r.Context(), r.PathValue("identifier"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revisions": revisions})
}

func (s *server) getSpecRevision(w http.ResponseWriter, r *http.Request) {
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil {
		writeError(w, app.ErrSpecNotFound)
		return
	}
	revision, err := s.service.GetSpecRevision(r.Context(), app.GetSpecRevision{Identifier: r.PathValue("identifier"), Number: number})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revision)
}
