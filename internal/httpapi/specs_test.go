package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
)

const specBody = `{"title":"Specs","status":"draft","stories":[],"edge_cases":[],"requirements":[{"id":"FR-001","text":"System MUST store specs."}],"entities":[],"success_criteria":[],"clarifications":[]}`

func TestSpecHTTP(t *testing.T) {
	root, _ := openService(t)
	handler := New(root, http.NotFoundHandler(), http.NotFoundHandler())
	request := func(method, path, body string, want int) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, "/api/w/default"+path, strings.NewReader(body)))
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	var spec app.Spec
	if err := json.Unmarshal(request("POST", "/specs", `{"team":"AL","message":"first","content":`+specBody+`}`, 201), &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Identifier != "SPEC-1" || spec.Revision != 1 || spec.Content.Requirements[0].ID != "FR-001" {
		t.Fatal(spec)
	}
	request("POST", "/specs", `{"team":"AL","content":`+strings.Replace(specBody, `"draft"`, `"final"`, 1)+`}`, 400)
	request("POST", "/specs", `{"team":"AL","author":"forged","content":`+specBody+`}`, 400)
	request("POST", "/specs", `{"team":"NOPE","content":`+specBody+`}`, 404)

	request("PUT", "/specs/SPEC-1", `{"base_revision":1,"message":"second","content":`+strings.Replace(specBody, `"draft"`, `"review"`, 1)+`}`, 200)
	request("PUT", "/specs/SPEC-1", `{"base_revision":1,"content":`+specBody+`}`, 409)
	request("PUT", "/specs/SPEC-1", `{"identifier":"SPEC-2","base_revision":2,"content":`+specBody+`}`, 400)
	request("PUT", "/specs/SPEC-9", `{"base_revision":1,"content":`+specBody+`}`, 404)

	if err := json.Unmarshal(request("GET", "/specs/SPEC-1", "", 200), &spec); err != nil || spec.Revision != 2 || spec.Content.Status != "review" {
		t.Fatal(spec, err)
	}
	var list struct{ Specs []app.SpecSummary }
	if err := json.Unmarshal(request("GET", "/specs?team=AL", "", 200), &list); err != nil || len(list.Specs) != 1 || list.Specs[0].Status != "review" {
		t.Fatal(list, err)
	}
	request("GET", "/specs?team=NOPE", "", 404)
	request("GET", "/specs?other=1", "", 400)
	var revisions struct{ Revisions []app.SpecRevisionSummary }
	if err := json.Unmarshal(request("GET", "/specs/SPEC-1/revisions", "", 200), &revisions); err != nil || len(revisions.Revisions) != 2 || revisions.Revisions[0].Number != 2 {
		t.Fatal(revisions, err)
	}
	var old app.SpecRevision
	if err := json.Unmarshal(request("GET", "/specs/SPEC-1/revisions/1", "", 200), &old); err != nil || old.Content.Status != "draft" || old.Message != "first" {
		t.Fatal(old, err)
	}
	request("GET", "/specs/SPEC-1/revisions/3", "", 404)
	request("GET", "/specs/SPEC-1/revisions/one", "", 404)
}
