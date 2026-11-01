package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxSpecContentBytes = 256 * 1024

// SpecContent follows GitHub Spec Kit's spec-template.md: prioritized user
// stories with Given/When/Then scenarios, functional requirements, key
// entities, success criteria and dated clarifications. Stories, requirements
// and criteria carry stable IDs (US-n, FR-n, SC-n).
type SpecContent struct {
	Title           string                     `json:"title" jsonschema:"required spec title"`
	Status          string                     `json:"status" jsonschema:"draft, review, approved or superseded"`
	Branch          string                     `json:"branch,omitempty" jsonschema:"optional Spec Kit feature branch name"`
	Stories         []SpecStory                `json:"stories"`
	EdgeCases       []string                   `json:"edge_cases"`
	Requirements    []SpecRequirement          `json:"requirements"`
	Entities        []SpecEntity               `json:"entities"`
	SuccessCriteria []SpecCriterion            `json:"success_criteria"`
	Clarifications  []SpecClarificationSession `json:"clarifications"`
}
type SpecStory struct {
	ID              string         `json:"id" jsonschema:"stable ID such as US-1"`
	Title           string         `json:"title"`
	Priority        string         `json:"priority" jsonschema:"P1, P2 or P3"`
	Description     string         `json:"description"`
	Why             string         `json:"why" jsonschema:"why this priority"`
	IndependentTest string         `json:"independent_test"`
	Scenarios       []SpecScenario `json:"scenarios"`
}
type SpecScenario struct {
	Given string `json:"given"`
	When  string `json:"when"`
	Then  string `json:"then"`
}
type SpecRequirement struct {
	ID      string `json:"id" jsonschema:"stable ID such as FR-001"`
	Text    string `json:"text"`
	Clarify string `json:"clarify,omitempty" jsonschema:"text of a NEEDS CLARIFICATION marker; empty when the requirement is clear"`
}
type SpecEntity struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
type SpecCriterion struct {
	ID   string `json:"id" jsonschema:"stable ID such as SC-001"`
	Text string `json:"text"`
}
type SpecClarificationSession struct {
	Date  string              `json:"date" jsonschema:"YYYY-MM-DD"`
	Items []SpecClarification `json:"items"`
}
type SpecClarification struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type SpecSummary struct {
	Identifier string `json:"identifier"`
	Team       string `json:"team"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Revision   int    `json:"revision"`
	UpdatedAt  string `json:"updated_at"`
}
type Spec struct {
	Identifier string      `json:"identifier"`
	Team       string      `json:"team"`
	Revision   int         `json:"revision"`
	Message    string      `json:"message" jsonschema:"change note of the current revision"`
	Author     *Identity   `json:"author" jsonschema:"author of the current revision"`
	CreatedAt  string      `json:"created_at"`
	UpdatedAt  string      `json:"updated_at"`
	Content    SpecContent `json:"content"`
}
type SpecRevisionSummary struct {
	Number    int       `json:"number"`
	Message   string    `json:"message"`
	Author    *Identity `json:"author"`
	CreatedAt string    `json:"created_at"`
}
type SpecRevision struct {
	SpecRevisionSummary
	Content SpecContent `json:"content"`
}

type CreateSpec struct {
	Team    string      `json:"team" jsonschema:"required team key; the spec is visible to whoever can read the team"`
	Content SpecContent `json:"content"`
	Message string      `json:"message,omitempty" jsonschema:"optional single-line change note, at most 240 characters"`
}
type UpdateSpec struct {
	Identifier   string      `json:"identifier" jsonschema:"spec identifier such as SPEC-1"`
	BaseRevision int         `json:"base_revision" jsonschema:"the revision this change was made from; rejected when it is no longer current"`
	Content      SpecContent `json:"content" jsonschema:"the complete new content; IDs removed in an earlier revision cannot be reused"`
	Message      string      `json:"message,omitempty" jsonschema:"optional single-line change note, at most 240 characters"`
}
type GetSpecRevision struct {
	Identifier string `json:"identifier" jsonschema:"spec identifier such as SPEC-1"`
	Number     int    `json:"number" jsonschema:"revision number from list_spec_revisions"`
}

var specStatuses = []string{"draft", "review", "approved", "superseded"}
var specIDPatterns = map[string]*regexp.Regexp{
	"story":       regexp.MustCompile(`^US-[0-9]+$`),
	"requirement": regexp.MustCompile(`^FR-[0-9]+$`),
	"criterion":   regexp.MustCompile(`^SC-[0-9]+$`),
}
var specIdentifier = regexp.MustCompile(`^SPEC-([1-9][0-9]*)$`)

func invalidSpec(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidSpec, fmt.Sprintf(format, args...))
}

func blank(values ...string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			return true
		}
	}
	return false
}

// validateSpec checks the content and returns the stable IDs it uses.
func validateSpec(c *SpecContent, message string) (map[string]bool, error) {
	if utf8.RuneCountInString(message) > 240 || !utf8.ValidString(message) || strings.ContainsFunc(message, unicode.IsControl) {
		return nil, invalidSpec("message must be a single line of at most 240 characters")
	}
	if blank(c.Title) {
		return nil, invalidSpec("title is required")
	}
	ok := false
	for _, status := range specStatuses {
		ok = ok || c.Status == status
	}
	if !ok {
		return nil, invalidSpec("status must be %s", joinOr(specStatuses))
	}
	ids := map[string]bool{}
	claim := func(kind, id string) error {
		if !specIDPatterns[kind].MatchString(id) {
			return invalidSpec("%s id %q is malformed", kind, id)
		}
		if ids[id] {
			return invalidSpec("duplicate id %s", id)
		}
		ids[id] = true
		return nil
	}
	for _, story := range c.Stories {
		if err := claim("story", story.ID); err != nil {
			return nil, err
		}
		if blank(story.Title, story.Description) || (story.Priority != "P1" && story.Priority != "P2" && story.Priority != "P3") {
			return nil, invalidSpec("%s needs a title, a description and priority P1, P2 or P3", story.ID)
		}
		for _, sc := range story.Scenarios {
			if blank(sc.Given, sc.When, sc.Then) {
				return nil, invalidSpec("%s has a scenario without given, when and then", story.ID)
			}
		}
	}
	for _, r := range c.Requirements {
		if err := claim("requirement", r.ID); err != nil {
			return nil, err
		}
		if blank(r.Text) {
			return nil, invalidSpec("%s needs text", r.ID)
		}
	}
	for _, sc := range c.SuccessCriteria {
		if err := claim("criterion", sc.ID); err != nil {
			return nil, err
		}
		if blank(sc.Text) {
			return nil, invalidSpec("%s needs text", sc.ID)
		}
	}
	for _, e := range c.Entities {
		if blank(e.Name, e.Description) {
			return nil, invalidSpec("every entity needs a name and a description")
		}
	}
	for _, session := range c.Clarifications {
		if _, err := time.Parse(time.DateOnly, session.Date); err != nil {
			return nil, invalidSpec("clarification date %q must be YYYY-MM-DD", session.Date)
		}
		for _, item := range session.Items {
			if blank(item.Question, item.Answer) {
				return nil, invalidSpec("every clarification needs a question and an answer")
			}
		}
	}
	return ids, nil
}

// encodeSpec stores empty lists as [] rather than null, then enforces the size limit.
func encodeSpec(c SpecContent) (string, error) {
	if c.Stories == nil {
		c.Stories = []SpecStory{}
	}
	for i := range c.Stories {
		if c.Stories[i].Scenarios == nil {
			c.Stories[i].Scenarios = []SpecScenario{}
		}
	}
	if c.EdgeCases == nil {
		c.EdgeCases = []string{}
	}
	if c.Requirements == nil {
		c.Requirements = []SpecRequirement{}
	}
	if c.Entities == nil {
		c.Entities = []SpecEntity{}
	}
	if c.SuccessCriteria == nil {
		c.SuccessCriteria = []SpecCriterion{}
	}
	if c.Clarifications == nil {
		c.Clarifications = []SpecClarificationSession{}
	}
	for i := range c.Clarifications {
		if c.Clarifications[i].Items == nil {
			c.Clarifications[i].Items = []SpecClarification{}
		}
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	if len(data) > maxSpecContentBytes {
		return "", invalidSpec("content is larger than %d KiB", maxSpecContentBytes/1024)
	}
	return string(data), nil
}

func (s *Service) CreateSpec(ctx context.Context, in CreateSpec) (Spec, error) {
	if err := s.checkAccess(ctx); err != nil {
		return Spec{}, err
	}
	if _, err := validateSpec(&in.Content, in.Message); err != nil {
		return Spec{}, err
	}
	content, err := encodeSpec(in.Content)
	if err != nil {
		return Spec{}, err
	}
	team, err := s.teamID(ctx, in.Team)
	if err != nil {
		return Spec{}, err
	}
	if !team.Valid {
		return Spec{}, ErrTeamRequired
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Spec{}, err
	}
	defer tx.Rollback()
	var number int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(number),0)+1 FROM specs WHERE workspace_id=?`, s.workspace).Scan(&number); err != nil {
		return Spec{}, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO specs(workspace_id,team_id,number,current_revision,created_at,updated_at) VALUES(?,?,?,1,?,?)`, s.workspace, team.Int64, number, now, now)
	if err != nil {
		return Spec{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Spec{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO spec_revisions(spec_id,number,content,message,author_subject,created_at) VALUES(?,1,?,?,NULLIF(?,''),?)`,
		id, content, in.Message, requestIdentity(ctx).Subject, now); err != nil {
		return Spec{}, err
	}
	if err = tx.Commit(); err != nil {
		return Spec{}, err
	}
	return s.GetSpec(ctx, "SPEC-"+strconv.Itoa(number))
}

// specRef finds a spec the caller can read; anything else is not found.
func (s *Service) specRef(ctx context.Context, identifier string) (id int64, current int, err error) {
	match := specIdentifier.FindStringSubmatch(identifier)
	if match == nil {
		return 0, 0, ErrSpecNotFound
	}
	query, args := viewerSQL(ctx, `SELECT id,current_revision FROM specs WHERE workspace_id=? AND number=? AND team_id IN (`+visibleTeamIDs+`)`, s.workspace, match[1])
	err = s.db.QueryRowContext(ctx, query, args...).Scan(&id, &current)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, ErrSpecNotFound
	}
	return id, current, err
}

func (s *Service) UpdateSpec(ctx context.Context, in UpdateSpec) (Spec, error) {
	if err := s.checkAccess(ctx); err != nil {
		return Spec{}, err
	}
	ids, err := validateSpec(&in.Content, in.Message)
	if err != nil {
		return Spec{}, err
	}
	content, err := encodeSpec(in.Content)
	if err != nil {
		return Spec{}, err
	}
	if in.BaseRevision < 1 {
		return Spec{}, invalidSpec("base_revision is required")
	}
	id, current, err := s.specRef(ctx, in.Identifier)
	if err != nil {
		return Spec{}, err
	}
	if current != in.BaseRevision {
		return Spec{}, ErrSpecConflict
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Spec{}, err
	}
	defer tx.Rollback()
	if err = rejectReusedSpecIDs(ctx, tx, id, current, ids); err != nil {
		return Spec{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE specs SET current_revision=?,updated_at=? WHERE id=? AND current_revision=?`, current+1, now, id, current)
	if err != nil {
		return Spec{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return Spec{}, err
	} else if changed != 1 {
		return Spec{}, ErrSpecConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO spec_revisions(spec_id,number,content,message,author_subject,created_at) VALUES(?,?,?,?,NULLIF(?,''),?)`,
		id, current+1, content, in.Message, requestIdentity(ctx).Subject, now); err != nil {
		return Spec{}, err
	}
	if err = tx.Commit(); err != nil {
		return Spec{}, err
	}
	return s.GetSpec(ctx, in.Identifier)
}

// rejectReusedSpecIDs refuses an ID that an earlier revision used and the
// current one dropped, so a stable ID never means two different things.
func rejectReusedSpecIDs(ctx context.Context, tx *sql.Tx, specID int64, current int, ids map[string]bool) error {
	rows, err := tx.QueryContext(ctx, `SELECT number,content FROM spec_revisions WHERE spec_id=?`, specID)
	if err != nil {
		return err
	}
	defer rows.Close()
	everUsed, now := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var number int
		var data string
		if err = rows.Scan(&number, &data); err != nil {
			return err
		}
		var old SpecContent
		if err = json.Unmarshal([]byte(data), &old); err != nil {
			return err
		}
		for _, story := range old.Stories {
			everUsed[story.ID], now[story.ID] = true, now[story.ID] || number == current
		}
		for _, r := range old.Requirements {
			everUsed[r.ID], now[r.ID] = true, now[r.ID] || number == current
		}
		for _, c := range old.SuccessCriteria {
			everUsed[c.ID], now[c.ID] = true, now[c.ID] || number == current
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for id := range ids {
		if everUsed[id] && !now[id] {
			return invalidSpec("%s was removed in an earlier revision and cannot be reused", id)
		}
	}
	return nil
}

const specRevisionColumns = `r.number,r.message,r.created_at,COALESCE(u.subject,''),COALESCE(u.name,''),COALESCE(u.email,'')`
const specRevisionFrom = ` FROM spec_revisions r LEFT JOIN users u ON u.subject=r.author_subject`

func scanSpecRevision(row interface{ Scan(...any) error }, content *string) (SpecRevisionSummary, error) {
	var rev SpecRevisionSummary
	var author Identity
	fields := []any{&rev.Number, &rev.Message, &rev.CreatedAt, &author.Subject, &author.Name, &author.Email}
	if content != nil {
		fields = append(fields, content)
	}
	err := row.Scan(fields...)
	if author.Subject != "" {
		rev.Author = &author
	}
	return rev, err
}

func (s *Service) GetSpec(ctx context.Context, identifier string) (Spec, error) {
	if err := s.checkAccess(ctx); err != nil {
		return Spec{}, err
	}
	id, current, err := s.specRef(ctx, identifier)
	if err != nil {
		return Spec{}, err
	}
	spec := Spec{Identifier: identifier, Revision: current}
	if err = s.db.QueryRowContext(ctx, `SELECT t.key,sp.created_at,sp.updated_at FROM specs sp JOIN teams t ON t.id=sp.team_id WHERE sp.id=?`, id).Scan(&spec.Team, &spec.CreatedAt, &spec.UpdatedAt); err != nil {
		return Spec{}, err
	}
	var content string
	rev, err := scanSpecRevision(s.db.QueryRowContext(ctx, `SELECT `+specRevisionColumns+`,r.content`+specRevisionFrom+` WHERE r.spec_id=? AND r.number=?`, id, current), &content)
	if err != nil {
		return Spec{}, err
	}
	spec.Message, spec.Author = rev.Message, rev.Author
	return spec, json.Unmarshal([]byte(content), &spec.Content)
}

func (s *Service) ListSpecs(ctx context.Context, team string) ([]SpecSummary, error) {
	if err := s.checkAccess(ctx); err != nil {
		return nil, err
	}
	teamID, err := s.teamID(ctx, team)
	if err != nil {
		return nil, err
	}
	query, args := viewerSQL(ctx, `SELECT sp.number,t.key,json_extract(r.content,'$.title'),json_extract(r.content,'$.status'),sp.current_revision,sp.updated_at
 FROM specs sp JOIN teams t ON t.id=sp.team_id JOIN spec_revisions r ON r.spec_id=sp.id AND r.number=sp.current_revision
 WHERE sp.workspace_id=? AND (?=0 OR sp.team_id=?) AND sp.team_id IN (`+visibleTeamIDs+`) ORDER BY sp.updated_at DESC,sp.id DESC`, s.workspace, teamID.Int64, teamID.Int64)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	specs := []SpecSummary{}
	for rows.Next() {
		var spec SpecSummary
		var number int
		if err = rows.Scan(&number, &spec.Team, &spec.Title, &spec.Status, &spec.Revision, &spec.UpdatedAt); err != nil {
			return nil, err
		}
		spec.Identifier = "SPEC-" + strconv.Itoa(number)
		specs = append(specs, spec)
	}
	return specs, rows.Err()
}

func (s *Service) ListSpecRevisions(ctx context.Context, identifier string) ([]SpecRevisionSummary, error) {
	if err := s.checkAccess(ctx); err != nil {
		return nil, err
	}
	id, _, err := s.specRef(ctx, identifier)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+specRevisionColumns+specRevisionFrom+` WHERE r.spec_id=? ORDER BY r.number DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	revisions := []SpecRevisionSummary{}
	for rows.Next() {
		rev, err := scanSpecRevision(rows, nil)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, rev)
	}
	return revisions, rows.Err()
}

func (s *Service) GetSpecRevision(ctx context.Context, in GetSpecRevision) (SpecRevision, error) {
	if err := s.checkAccess(ctx); err != nil {
		return SpecRevision{}, err
	}
	id, _, err := s.specRef(ctx, in.Identifier)
	if err != nil {
		return SpecRevision{}, err
	}
	var result SpecRevision
	var content string
	result.SpecRevisionSummary, err = scanSpecRevision(s.db.QueryRowContext(ctx, `SELECT `+specRevisionColumns+`,r.content`+specRevisionFrom+` WHERE r.spec_id=? AND r.number=?`, id, in.Number), &content)
	if errors.Is(err, sql.ErrNoRows) {
		return SpecRevision{}, ErrSpecNotFound
	}
	if err != nil {
		return SpecRevision{}, err
	}
	return result, json.Unmarshal([]byte(content), &result.Content)
}
