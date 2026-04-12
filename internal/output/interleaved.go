package output

import (
	"io"

	"github.com/clintmod/rite/internal/templater"
)

type Interleaved struct{}

func (Interleaved) WrapWriter(stdOut, stdErr io.Writer, _ string, _ *templater.Cache) (io.Writer, io.Writer, CloseFunc) {
	return stdOut, stdErr, func(error) error { return nil }
}
