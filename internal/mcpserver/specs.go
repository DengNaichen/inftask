package mcpserver

import (
	"context"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func addSpecTools(server *mcp.Server, service *app.Service) {
	type identifierInput struct {
		Identifier string `json:"identifier" jsonschema:"spec identifier such as SPEC-1"`
	}
	type listInput struct {
		Team string `json:"team,omitempty" jsonschema:"team key; omitted lists every readable team"`
	}
	type listOutput struct {
		Specs []app.SpecSummary `json:"specs"`
	}
	type revisionsOutput struct {
		Revisions []app.SpecRevisionSummary `json:"revisions" jsonschema:"newest first"`
	}
	const format = " Content follows GitHub Spec Kit's spec template: prioritized user stories (US-n) with Given/When/Then scenarios, edge cases, functional requirements (FR-n, with an optional clarify text for NEEDS CLARIFICATION), entities, success criteria (SC-n) and dated clarifications."
	addTool(server, &mcp.Tool{Name: "list_specs", Description: "List spec summaries the caller can read, newest updated first, optionally for one team. Specs are the long-lived description of what the system should be; issues are the changes that implement them."}, func(ctx context.Context, _ *mcp.CallToolRequest, input listInput) (*mcp.CallToolResult, listOutput, error) {
		specs, err := service.ListSpecs(ctx, input.Team)
		return nil, listOutput{specs}, err
	})
	addTool(server, &mcp.Tool{Name: "get_spec", Description: "Read the current revision of a spec, including its full structured content." + format}, func(ctx context.Context, _ *mcp.CallToolRequest, input identifierInput) (*mcp.CallToolResult, app.Spec, error) {
		spec, err := service.GetSpec(ctx, input.Identifier)
		return nil, spec, err
	})
	addTool(server, &mcp.Tool{Name: "create_spec", Description: "Create a spec in a team as revision 1. Visible to whoever can read the team." + format}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.CreateSpec) (*mcp.CallToolResult, app.Spec, error) {
		spec, err := service.CreateSpec(ctx, input)
		return nil, spec, err
	})
	addTool(server, &mcp.Tool{Name: "update_spec", Description: "Save a spec as a new revision. Supply the complete new content and the base_revision you read it at; the save is rejected with a conflict if another revision was saved since, so read the spec again and retry. An ID removed in an earlier revision cannot be reused. Revisions are append-only." + format}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.UpdateSpec) (*mcp.CallToolResult, app.Spec, error) {
		spec, err := service.UpdateSpec(ctx, input)
		return nil, spec, err
	})
	addTool(server, &mcp.Tool{Name: "list_spec_revisions", Description: "List a spec's revisions, newest first, with author and change note. Use get_spec_revision to read one."}, func(ctx context.Context, _ *mcp.CallToolRequest, input identifierInput) (*mcp.CallToolResult, revisionsOutput, error) {
		revisions, err := service.ListSpecRevisions(ctx, input.Identifier)
		return nil, revisionsOutput{revisions}, err
	})
	addTool(server, &mcp.Tool{Name: "get_spec_revision", Description: "Read one historical revision of a spec with its full content."}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.GetSpecRevision) (*mcp.CallToolResult, app.SpecRevision, error) {
		revision, err := service.GetSpecRevision(ctx, input)
		return nil, revision, err
	})
}
