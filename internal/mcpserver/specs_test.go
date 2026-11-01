package mcpserver

import (
	"context"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSpecMCP(t *testing.T) {
	session := connect(t)
	content := map[string]any{"title": "Specs", "status": "draft", "stories": []any{}, "edge_cases": []any{},
		"requirements": []any{map[string]any{"id": "FR-001", "text": "System MUST store specs."}}, "entities": []any{}, "success_criteria": []any{}, "clarifications": []any{}}
	var spec app.Spec
	call(t, session, "create_spec", map[string]any{"team": "AL", "content": content, "message": "first"}, &spec)
	if spec.Identifier != "SPEC-1" || spec.Revision != 1 {
		t.Fatal(spec)
	}
	content["status"] = "review"
	call(t, session, "update_spec", map[string]any{"identifier": "SPEC-1", "base_revision": 1, "content": content, "message": "second"}, &spec)
	if spec.Revision != 2 || spec.Content.Status != "review" {
		t.Fatal(spec)
	}
	call(t, session, "get_spec", map[string]any{"identifier": "SPEC-1"}, &spec)
	var list struct{ Specs []app.SpecSummary }
	call(t, session, "list_specs", map[string]any{"team": "AL"}, &list)
	if len(list.Specs) != 1 || list.Specs[0].Revision != 2 {
		t.Fatal(list)
	}
	var revisions struct{ Revisions []app.SpecRevisionSummary }
	call(t, session, "list_spec_revisions", map[string]any{"identifier": "SPEC-1"}, &revisions)
	if len(revisions.Revisions) != 2 {
		t.Fatal(revisions)
	}
	var old app.SpecRevision
	call(t, session, "get_spec_revision", map[string]any{"identifier": "SPEC-1", "number": 1}, &old)
	if old.Content.Status != "draft" {
		t.Fatal(old)
	}
	for name, args := range map[string]map[string]any{
		"update_spec": {"identifier": "SPEC-1", "base_revision": 1, "content": content},
		"get_spec":    {"identifier": "SPEC-1", "actor": "forged"},
		"create_spec": {"team": "NOPE", "content": content},
	} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err == nil && !result.IsError {
			t.Fatal(name, "accepted", args)
		}
	}
}
