package errors

import (
	"fmt"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
)

// RitefileNotFoundError is returned when no appropriate Ritefile is found when
// searching the filesystem.
type RitefileNotFoundError struct {
	URI         string
	Walk        bool
	AskInit     bool
	OwnerChange bool
}

func (err RitefileNotFoundError) Error() string {
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

func (err RitefileNotFoundError) Code() int {
	return CodeRitefileNotFound
}

// RitefileAlreadyExistsError is returned on creating a Ritefile if one already
// exists.
type RitefileAlreadyExistsError struct{}

func (err RitefileAlreadyExistsError) Error() string {
	return "rite: A Ritefile already exists"
}

func (err RitefileAlreadyExistsError) Code() int {
	return CodeRitefileAlreadyExists
}

// RitefileInvalidError is returned when the Ritefile contains syntax errors or
// cannot be parsed for some reason.
type RitefileInvalidError struct {
	URI string
	Err error
}

func (err RitefileInvalidError) Error() string {
	return fmt.Sprintf("rite: Failed to parse %s:\n%v", filepath.ToSlash(err.URI), err.Err)
}

func (err RitefileInvalidError) Code() int {
	return CodeRitefileInvalid
}

// RitefileVersionCheckError is returned when the user attempts to run a
// Ritefile that does not contain a schema version key or if they try to use a
// feature that is not supported by the schema version.
type RitefileVersionCheckError struct {
	URI           string
	SchemaVersion *semver.Version
	Message       string
}

func (err *RitefileVersionCheckError) Error() string {
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

func (err *RitefileVersionCheckError) Code() int {
	return CodeRitefileVersionCheckError
}

// RitefileCycleError is returned when we detect that a Ritefile includes a
// set of Ritefiles that include each other in a cycle.
type RitefileCycleError struct {
	Source      string
	Destination string
}

func (err RitefileCycleError) Error() string {
	return fmt.Sprintf("rite: include cycle detected between %s <--> %s",
		filepath.ToSlash(err.Source),
		filepath.ToSlash(err.Destination),
	)
}

func (err RitefileCycleError) Code() int {
	return CodeRitefileCycle
}

// RitefileDoesNotMatchChecksum is returned when a Ritefile's checksum does not
// match the one pinned in the parent Ritefile.
type RitefileDoesNotMatchChecksum struct {
	URI              string
	ExpectedChecksum string
	ActualChecksum   string
}

func (err *RitefileDoesNotMatchChecksum) Error() string {
	return fmt.Sprintf(
		"rite: The checksum of the Ritefile at %q does not match!\ngot: %q\nwant: %q",
		filepath.ToSlash(err.URI),
		err.ActualChecksum,
		err.ExpectedChecksum,
	)
}

func (err *RitefileDoesNotMatchChecksum) Code() int {
	return CodeRitefileDoesNotMatchChecksum
}
