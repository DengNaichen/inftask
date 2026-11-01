package app

import (
	"errors"
	"slices"
)

// Kind says why an operation rejected a request, so transports can choose a
// status code without listing every error.
type Kind int

const (
	// KindInternal is the kind of every error that is not one of the Err
	// variables, such as a database failure.
	KindInternal Kind = iota
	KindNotFound
	KindInvalid
	KindConflict
	KindForbidden
)

// kindError is an error of a Kind. Each Err variable is a distinct pointer, so
// errors.Is still matches it only by identity.
type kindError struct {
	kind    Kind
	message string
}

func (e *kindError) Error() string { return e.message }

func newError(kind Kind, message string) error { return &kindError{kind, message} }

// KindOf returns the Kind of the first Err variable in err's chain, including
// one wrapped with %w, or KindInternal when there is none.
func KindOf(err error) Kind {
	var classified *kindError
	if errors.As(err, &classified) {
		return classified.kind
	}
	return KindInternal
}

var (
	ErrCycleNotFound  = newError(KindNotFound, "cycle not found")
	ErrCycleSettings  = newError(KindInvalid, "use a valid IANA timezone, start date, 1–8 weeks and 1–15 upcoming cycles")
	ErrCycleSchedule  = newError(KindConflict, "disable cycles before changing the schedule; historical cycles are preserved")
	ErrCycleClosed    = newError(KindConflict, "cannot assign an issue to a closed or disabled cycle")
	ErrCycleReference = newError(KindInvalid, "cycle_id must be zero or a positive cycle id")

	ErrProjectDates         = newError(KindInvalid, "use valid YYYY-MM-DD dates with start on or before target")
	ErrInitiativeNotFound   = newError(KindNotFound, "initiative not found")
	ErrInitiativeExists     = newError(KindConflict, "an initiative with this name already exists")
	ErrInitiativeArchived   = newError(KindConflict, "restore the initiative and project before adding an association")
	ErrNotFound             = newError(KindNotFound, "issue not found")
	ErrInvalidName          = newError(KindInvalid, "name is required")
	ErrProjectExists        = newError(KindConflict, "a project with this name already exists")
	ErrProjectTeamsRequired = newError(KindInvalid, "teams is required and must contain at least one Team key")
	ErrProjectReference     = newError(KindInvalid, "project id and name must identify the same Project")
	ErrProjectTeam          = newError(KindConflict, "project is not associated with the issue Team")
	ErrProjectTeamInUse     = newError(KindConflict, "cannot remove a Team that still has issues in this Project")
	ErrProjectArchived      = newError(KindConflict, "cannot add an issue to an archived Project")
	ErrProjectNotFound      = newError(KindNotFound, "project not found")
	ErrInvalidTitle         = newError(KindInvalid, "title is required")
	ErrInvalidStatus        = newError(KindInvalid, "invalid status: use "+joinOr(statusNames()))
	ErrInvalidPriority      = newError(KindInvalid, "invalid priority: use 0 (no priority), 1 (urgent), 2 (high), 3 (medium), or 4 (low)")
	ErrInvalidSort          = newError(KindInvalid, "invalid sort: use priority or updated")
	ErrInvalidType          = newError(KindInvalid, "invalid type: use "+joinOr(append(slices.Clone(Types), "an empty string")))
	ErrInvalidKind          = newError(KindInvalid, "invalid relation kind: use blocks, blocked_by, or related")
	ErrRelationCycle        = newError(KindInvalid, "dependency cycle is not allowed")
	ErrTeamRequired         = newError(KindInvalid, "team is required")
	ErrTeamNotFound         = newError(KindNotFound, "team not found")
	ErrInvalidTeam          = newError(KindInvalid, "invalid team: key must be 2 to 6 uppercase letters or digits starting with a letter, name is required, and start_number must be at least 1")
	ErrTeamExists           = newError(KindConflict, "a team with this key already exists")

	ErrWorkspaceNotFound = newError(KindNotFound, "workspace not found")
	ErrInvalidWorkspace  = newError(KindInvalid, "invalid workspace: slug must be 2 to 32 lowercase letters, digits, or hyphens starting with a letter and cannot be api, assets, auth, mcp, w, or webhooks; name is required")
	ErrWorkspaceExists   = newError(KindConflict, "a workspace with this slug already exists")

	ErrSpecNotFound = newError(KindNotFound, "spec not found")
	ErrSpecConflict = newError(KindConflict, "spec changed since base_revision; read the current revision and retry")
	ErrInvalidSpec  = newError(KindInvalid, "invalid spec")

	ErrInvalidEmoji = newError(KindInvalid, "invalid emoji: choose a Unicode emoji or use an empty string to clear")

	ErrInvalidLabel  = newError(KindInvalid, "invalid label operation")
	ErrLabelNotFound = newError(KindNotFound, "label or label group not found")
)
