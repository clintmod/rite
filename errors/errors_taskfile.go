package errors

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
)

// TaskfileNotFoundError is returned when no appropriate Taskfile is found when
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

// TaskfileAlreadyExistsError is returned on creating a Taskfile if one already
// exists.
type TaskfileAlreadyExistsError struct{}

func (err TaskfileAlreadyExistsError) Error() string {
	return "rite: A Ritefile already exists"
}

func (err TaskfileAlreadyExistsError) Code() int {
	return CodeTaskfileAlreadyExists
}

// TaskfileInvalidError is returned when the Taskfile contains syntax errors or
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

// TaskfileFetchFailedError is returned when no appropriate Taskfile is found when
// searching the filesystem.
type TaskfileFetchFailedError struct {
	URI            string
	HTTPStatusCode int
}

func (err TaskfileFetchFailedError) Error() string {
	var statusText string
	if err.HTTPStatusCode != 0 {
		statusText = fmt.Sprintf(" with status code %d (%s)", err.HTTPStatusCode, http.StatusText(err.HTTPStatusCode))
	}
	return fmt.Sprintf(`rite: Download of %q failed%s`, filepath.ToSlash(err.URI), statusText)
}

func (err TaskfileFetchFailedError) Code() int {
	return CodeTaskfileFetchFailed
}

// TaskfileNotTrustedError is returned when the user does not accept the trust
// prompt when downloading a remote Taskfile.
type TaskfileNotTrustedError struct {
	URI string
}

func (err *TaskfileNotTrustedError) Error() string {
	return fmt.Sprintf(
		`rite: Ritefile %q not trusted by user`,
		filepath.ToSlash(err.URI),
	)
}

func (err *TaskfileNotTrustedError) Code() int {
	return CodeTaskfileNotTrusted
}

// TaskfileNotSecureError is returned when the user attempts to download a
// remote Taskfile over an insecure connection.
type TaskfileNotSecureError struct {
	URI string
}

func (err *TaskfileNotSecureError) Error() string {
	return fmt.Sprintf(
		`rite: Ritefile %q cannot be downloaded over an insecure connection. You can override this by using the --insecure flag`,
		filepath.ToSlash(err.URI),
	)
}

func (err *TaskfileNotSecureError) Code() int {
	return CodeTaskfileNotSecure
}

// TaskfileCacheNotFoundError is returned when the user attempts to use an offline
// (cached) Taskfile but the files does not exist in the cache.
type TaskfileCacheNotFoundError struct {
	URI string
}

func (err *TaskfileCacheNotFoundError) Error() string {
	return fmt.Sprintf(
		`rite: Ritefile %q was not found in the cache. Remove the --offline flag to use a remote copy or download it using the --download flag`,
		filepath.ToSlash(err.URI),
	)
}

func (err *TaskfileCacheNotFoundError) Code() int {
	return CodeTaskfileCacheNotFound
}

// TaskfileVersionCheckError is returned when the user attempts to run a
// Taskfile that does not contain a Taskfile schema version key or if they try
// to use a feature that is not supported by the schema version.
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

// TaskfileNetworkTimeoutError is returned when the user attempts to use a remote
// Taskfile but a network connection could not be established within the timeout.
type TaskfileNetworkTimeoutError struct {
	URI     string
	Timeout time.Duration
}

func (err *TaskfileNetworkTimeoutError) Error() string {
	return fmt.Sprintf(
		`rite: Network connection timed out after %s while attempting to download Ritefile %q`,
		err.Timeout, filepath.ToSlash(err.URI),
	)
}

func (err *TaskfileNetworkTimeoutError) Code() int {
	return CodeTaskfileNetworkTimeout
}

// TaskfileCycleError is returned when we detect that a Taskfile includes a
// set of Taskfiles that include each other in a cycle.
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

// TaskfileDoesNotMatchChecksum is returned when a Taskfile's checksum does not
// match the one pinned in the parent Taskfile.
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
