package errors

import "fmt"

type RitercNotFoundError struct {
	URI  string
	Walk bool
}

func (err RitercNotFoundError) Error() string {
	var walkText string
	if err.Walk {
		walkText = " (or any of the parent directories)"
	}
	return fmt.Sprintf(`rite: No Task config file found at %q%s`, err.URI, walkText)
}

func (err RitercNotFoundError) Code() int {
	return CodeRitercNotFoundError
}
