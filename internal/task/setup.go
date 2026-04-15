package task

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/sajari/fuzzy"

	"github.com/clintmod/rite/errors"
	"github.com/clintmod/rite/internal/env"
	"github.com/clintmod/rite/internal/execext"
	"github.com/clintmod/rite/internal/filepathext"
	"github.com/clintmod/rite/internal/logger"
	"github.com/clintmod/rite/internal/output"
	"github.com/clintmod/rite/taskfile"
	"github.com/clintmod/rite/taskfile/ast"
)

func (e *Executor) Setup() error {
	e.setupLogger()
	node, err := e.getRootNode()
	if err != nil {
		return err
	}
	if err := e.setupTempDir(); err != nil {
		return err
	}
	if err := e.readTaskfile(node); err != nil {
		return err
	}
	e.setupStdFiles()
	if err := e.setupTimestamps(); err != nil {
		return err
	}
	if err := e.setupOutput(); err != nil {
		return err
	}
	if err := e.setupCompiler(); err != nil {
		return err
	}
	if err := e.readDotEnvFiles(); err != nil {
		return err
	}
	if err := e.doVersionChecks(); err != nil {
		return err
	}
	e.setupDefaults()
	e.setupConcurrencyState()
	return nil
}

func (e *Executor) getRootNode() (taskfile.Node, error) {
	node, err := taskfile.NewRootNode(e.Entrypoint, e.Dir)
	var taskNotFoundError errors.RitefileNotFoundError
	if errors.As(err, &taskNotFoundError) {
		taskNotFoundError.AskInit = true
		return nil, taskNotFoundError
	}
	if err != nil {
		return nil, err
	}
	e.Dir = node.Dir()
	e.Entrypoint = node.Location()
	return node, err
}

func (e *Executor) readTaskfile(node taskfile.Node) error {
	debugFunc := func(s string) {
		e.Logger.VerboseOutf(logger.Magenta, s)
	}
	reader := taskfile.NewReader(
		taskfile.WithDebugFunc(debugFunc),
	)
	graph, err := reader.Read(context.Background(), node)
	if err != nil {
		return err
	}
	if e.Ritefile, err = graph.Merge(); err != nil {
		return err
	}
	return nil
}

func (e *Executor) setupFuzzyModel() {
	if e.Ritefile == nil {
		return
	}

	model := fuzzy.NewModel()
	model.SetThreshold(1) // because we want to build grammar based on every task name

	var words []string
	for name, task := range e.Ritefile.Tasks.All(nil) {
		if task.Internal {
			continue
		}
		words = append(words, name)
		words = slices.Concat(words, task.Aliases)
	}

	model.Train(words)
	e.fuzzyModel = model
}

func (e *Executor) setupTempDir() error {
	if e.TempDir != (TempDir{}) {
		return nil
	}

	tempDir := env.GetRiteEnv("TEMP_DIR")
	if tempDir == "" {
		e.TempDir = TempDir{
			Fingerprint: filepathext.SmartJoin(e.Dir, ".rite"),
		}
	} else if filepath.IsAbs(tempDir) || strings.HasPrefix(tempDir, "~") {
		tempDir, err := execext.ExpandLiteral(tempDir)
		if err != nil {
			return err
		}
		projectDir, _ := filepath.Abs(e.Dir)
		projectName := filepath.Base(projectDir)
		e.TempDir = TempDir{
			Fingerprint: filepathext.SmartJoin(tempDir, projectName),
		}
	} else {
		e.TempDir = TempDir{
			Fingerprint: filepathext.SmartJoin(e.Dir, tempDir),
		}
	}

	return nil
}

func (e *Executor) setupStdFiles() {
	if e.Stdin == nil {
		e.Stdin = os.Stdin
	}
	if e.Stdout == nil {
		e.Stdout = os.Stdout
	}
	if e.Stderr == nil {
		e.Stderr = os.Stderr
	}
}

// setupTimestamps resolves CLI/top-level timestamp configuration and, if
// timestamps are globally on, wraps the Executor's Stdout/Stderr before the
// logger captures them. Must run after readTaskfile (needs e.Ritefile) and
// setupStdFiles (needs e.Stdout / e.Stderr) but before setupLogger (the
// logger snapshots e.Stdout / e.Stderr).
func (e *Executor) setupTimestamps() error {
	tc, err := e.buildTimestampContext()
	if err != nil {
		return err
	}
	e.tsCtx = tc
	// Route rite's own logger through the timestamping sinks when the
	// global scope (CLI > top-level) is on. We intentionally leave
	// e.Stdout / e.Stderr *unwrapped* — cmd output is timestamped
	// per-task by wrapCmdWriters() using the effective per-task layout,
	// which may differ from the global layout (per-task strftime
	// override) or be disabled entirely (task-level `timestamps: false`
	// opt-out against a global-on setting). Wrapping e.Stdout globally
	// would force every cmd line through the global layout and make the
	// per-task override unreachable.
	if e.Logger != nil {
		loggerOut, loggerErr, closer := tc.wrapLoggerWriters(e.Logger.Stdout, e.Logger.Stderr)
		e.Logger.Stdout = loggerOut
		e.Logger.Stderr = loggerErr
		e.tsCloseLoggers = closer
	} else {
		e.tsCloseLoggers = func() {}
	}
	return nil
}

func (e *Executor) setupLogger() {
	e.Logger = &logger.Logger{
		Stdin:      e.Stdin,
		Stdout:     e.Stdout,
		Stderr:     e.Stderr,
		Verbose:    e.Verbose,
		Color:      e.Color,
		AssumeYes:  e.AssumeYes,
		AssumeTerm: e.AssumeTerm,
	}
}

func (e *Executor) setupOutput() error {
	if !e.OutputStyle.IsSet() {
		e.OutputStyle = e.Ritefile.Output
	}

	var err error
	e.Output, err = output.BuildFor(&e.OutputStyle, e.Logger)
	return err
}

func (e *Executor) setupCompiler() error {
	if e.UserWorkingDir == "" {
		var err error
		e.UserWorkingDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	e.Compiler = &Compiler{
		Dir:            e.Dir,
		Entrypoint:     e.Entrypoint,
		UserWorkingDir: e.UserWorkingDir,
		RitefileEnv:    e.Ritefile.Env,
		RitefileVars:   e.Ritefile.Vars,
		Logger:         e.Logger,
	}
	return nil
}

func (e *Executor) readDotEnvFiles() error {
	if e.Ritefile == nil || len(e.Ritefile.Dotenv) == 0 {
		return nil
	}

	if e.Ritefile.Version.LessThan(ast.V3) {
		return nil
	}

	vars, err := e.Compiler.GetRitefileVariables()
	if err != nil {
		return err
	}

	env, err := taskfile.Dotenv(vars, e.Ritefile, e.Dir)
	if err != nil {
		return err
	}

	for k, v := range env.All() {
		if _, ok := e.Ritefile.Env.Get(k); !ok {
			e.Ritefile.Env.Set(k, v)
		}
	}
	return err
}

func (e *Executor) setupDefaults() {
	if e.Ritefile.Method == "" {
		e.Ritefile.Method = "checksum"
	}
	if e.Ritefile.Run == "" {
		e.Ritefile.Run = "always"
	}
}

func (e *Executor) setupConcurrencyState() {
	e.executionHashes = make(map[string]context.Context)

	e.taskCallCount = make(map[string]*int32, e.Ritefile.Tasks.Len())
	e.mkdirMutexMap = make(map[string]*sync.Mutex, e.Ritefile.Tasks.Len())
	for k := range e.Ritefile.Tasks.Keys(nil) {
		e.taskCallCount[k] = new(int32)
		e.mkdirMutexMap[k] = &sync.Mutex{}
	}

	if e.Concurrency > 0 {
		e.concurrencySemaphore = make(chan struct{}, e.Concurrency)
	}
}

func (e *Executor) doVersionChecks() error {
	if !e.EnableVersionCheck {
		return nil
	}
	// Copy the version to avoid modifying the original
	schemaVersion := &semver.Version{}
	*schemaVersion = *e.Ritefile.Version

	// Error if the Ritefile uses a schema version below v3
	if schemaVersion.LessThan(ast.V3) {
		return &errors.RitefileVersionCheckError{
			URI:           e.Ritefile.Location,
			SchemaVersion: schemaVersion,
			Message:       `no longer supported. Please use v3 or above`,
		}
	}

	// Error if the schema version is at or above the next major. Without this
	// bound, `version: '4'` / `version: '999'` silently parse and run under v3
	// semantics. See issue #46.
	if schemaVersion.GreaterThanEqual(ast.V4) {
		return &errors.RitefileVersionCheckError{
			URI:           e.Ritefile.Location,
			SchemaVersion: schemaVersion,
			Message:       `is not supported. rite currently supports schema version 3`,
		}
	}

	return nil
}
