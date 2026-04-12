package experiments

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/clintmod/rite/internal/slicesext"
)

type InvalidValueError struct {
	Name          string
	AllowedValues []int
	Value         int
}

func (err InvalidValueError) Error() string {
	return fmt.Sprintf(
		"rite: Experiment %q has an invalid value %q (allowed values: %s)",
		err.Name,
		err.Value,
		strings.Join(slicesext.Convert(err.AllowedValues, strconv.Itoa), ", "),
	)
}

type InactiveError struct {
	Name string
}

func (err InactiveError) Error() string {
	return fmt.Sprintf(
		"rite: Experiment %q is inactive and cannot be enabled",
		err.Name,
	)
}
