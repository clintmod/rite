package errors

import (
	"fmt"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
)

// TaskfileNotFoundError is returned when no appropriate Ritefile is found when
// searching the filesystem.
type TaskfileNotFoundError struct {
	URI         string
	Walk        bool
	AskInit     bool
	OwnerChange bool
}

func (err TaskfileNotFoundError) Error() string {
	var walkText string
	if err.OwnerChange {
		walkText = " (or any of the parent directories until ownership changed)."
	} else if err.Walk {
		walkText = " (or any of the parent directories)."
	}
	if err.AskInit {
		walkText += " Run `rite --init` to create a new Ritefile."
	}
	return fmt.Sprintf(`rite: No Ritefile found at %q%s`, filepath.ToSlash(err.URI), walkText)
}

func (err TaskfileNotFoundError) Code() int {
	return CodeTaskfileNotFound
}

// TaskfileAlreadyExistsError is returned on creating a Ritefile if one already
// exists.
type TaskfileAlreadyExistsError struct{}

func (err TaskfileAlreadyExistsError) Error() string {
	return "rite: A Ritefile already exists"
}

func (err TaskfileAlreadyExistsError) Code() int {
	return CodeTaskfileAlreadyExists
}

// TaskfileInvalidError is returned when the Ritefile contains syntax errors or
// cannot be parsed for some reason.
type TaskfileInvalidError struct {
	URI string
	Err error
}

func (err TaskfileInvalidError) Error() string {
	return fmt.Sprintf("rite: Failed to parse %s:\n%v", filepath.ToSlash(err.URI), err.Err)
}

func (err TaskfileInvalidError) Code() int {
	return CodeTaskfileInvalid
}

// TaskfileVersionCheckError is returned when the user attempts to run a
// Ritefile that does not contain a schema version key or if they try to use a
// feature that is not supported by the schema version.
type TaskfileVersionCheckError struct {
	URI           string
	SchemaVersion *semver.Version
	Message       string
}

func (err *TaskfileVersionCheckError) Error() string {
	if err.SchemaVersion == nil {
		return fmt.Sprintf(
			`rite: Missing schema version in Ritefile %q`,
			filepath.ToSlash(err.URI),
		)
	}
	return fmt.Sprintf(
		"rite: Invalid schema version in Ritefile %q:\nSchema version (%s) %s",
		filepath.ToSlash(err.URI),
		err.SchemaVersion.String(),
		err.Message,
	)
}

func (err *TaskfileVersionCheckError) Code() int {
	return CodeTaskfileVersionCheckError
}

// TaskfileCycleError is returned when we detect that a Ritefile includes a
// set of Ritefiles that include each other in a cycle.
type TaskfileCycleError struct {
	Source      string
	Destination string
}

func (err TaskfileCycleError) Error() string {
	return fmt.Sprintf("rite: include cycle detected between %s <--> %s",
		filepath.ToSlash(err.Source),
		filepath.ToSlash(err.Destination),
	)
}

func (err TaskfileCycleError) Code() int {
	return CodeTaskfileCycle
}

// TaskfileDoesNotMatchChecksum is returned when a Ritefile's checksum does not
// match the one pinned in the parent Ritefile.
type TaskfileDoesNotMatchChecksum struct {
	URI              string
	ExpectedChecksum string
	ActualChecksum   string
}

func (err *TaskfileDoesNotMatchChecksum) Error() string {
	return fmt.Sprintf(
		"rite: The checksum of the Ritefile at %q does not match!\ngot: %q\nwant: %q",
		filepath.ToSlash(err.URI),
		err.ActualChecksum,
		err.ExpectedChecksum,
	)
}

func (err *TaskfileDoesNotMatchChecksum) Code() int {
	return CodeTaskfileDoesNotMatchChecksum
}
