package fingerprint

import (
	"context"

	"github.com/clintmod/rite/internal/env"
	"github.com/clintmod/rite/internal/execext"
	"github.com/clintmod/rite/internal/logger"
	"github.com/clintmod/rite/taskfile/ast"
)

type StatusChecker struct {
	logger *logger.Logger
}

func NewStatusChecker(logger *logger.Logger) StatusCheckable {
	return &StatusChecker{
		logger: logger,
	}
}

func (checker *StatusChecker) IsUpToDate(ctx context.Context, t *ast.Task) (bool, error) {
	for _, s := range t.Status {
		err := execext.RunCommand(ctx, &execext.RunCommandOptions{
			Command: s,
			Dir:     t.Dir,
			Env:     env.Get(t),
		})
		if err != nil {
			checker.logger.VerboseOutf(logger.Yellow, "task: status command %s exited non-zero: %s\n", s, err)
			return false, nil
		}
		checker.logger.VerboseOutf(logger.Yellow, "task: status command %s exited zero\n", s)
	}
	return true, nil
}
