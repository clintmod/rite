package task

import (
	"context"
	"fmt"
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
	"github.com/clintmod/rite/internal/version"
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
	var taskNotFoundError errors.TaskfileNotFoundError
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
	if e.Taskfile, err = graph.Merge(); err != nil {
		return err
	}
	return nil
}

func (e *Executor) setupFuzzyModel() {
	if e.Taskfile == nil {
		return
	}

	model := fuzzy.NewModel()
	model.SetThreshold(1) // because we want to build grammar based on every task name

	var words []string
	for name, task := range e.Taskfile.Tasks.All(nil) {
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
		e.OutputStyle = e.Taskfile.Output
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
		TaskfileEnv:    e.Taskfile.Env,
		TaskfileVars:   e.Taskfile.Vars,
		Logger:         e.Logger,
	}
	return nil
}

func (e *Executor) readDotEnvFiles() error {
	if e.Taskfile == nil || len(e.Taskfile.Dotenv) == 0 {
		return nil
	}

	if e.Taskfile.Version.LessThan(ast.V3) {
		return nil
	}

	vars, err := e.Compiler.GetTaskfileVariables()
	if err != nil {
		return err
	}

	env, err := taskfile.Dotenv(vars, e.Taskfile, e.Dir)
	if err != nil {
		return err
	}

	for k, v := range env.All() {
		if _, ok := e.Taskfile.Env.Get(k); !ok {
			e.Taskfile.Env.Set(k, v)
		}
	}
	return err
}

func (e *Executor) setupDefaults() {
	if e.Taskfile.Method == "" {
		e.Taskfile.Method = "checksum"
	}
	if e.Taskfile.Run == "" {
		e.Taskfile.Run = "always"
	}
}

func (e *Executor) setupConcurrencyState() {
	e.executionHashes = make(map[string]context.Context)

	e.taskCallCount = make(map[string]*int32, e.Taskfile.Tasks.Len())
	e.mkdirMutexMap = make(map[string]*sync.Mutex, e.Taskfile.Tasks.Len())
	for k := range e.Taskfile.Tasks.Keys(nil) {
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
	*schemaVersion = *e.Taskfile.Version

	// Error if the Taskfile uses a schema version below v3
	if schemaVersion.LessThan(ast.V3) {
		return &errors.TaskfileVersionCheckError{
			URI:           e.Taskfile.Location,
			SchemaVersion: schemaVersion,
			Message:       `no longer supported. Please use v3 or above`,
		}
	}

	// Get the current version of Task
	// If we can't parse the version (e.g. when its "devel"), then ignore the current version checks
	currentVersion, err := semver.NewVersion(version.GetVersion())
	if err != nil {
		return nil
	}

	// Error if the Taskfile uses a schema version above the current version of Task
	if schemaVersion.GreaterThan(currentVersion) {
		return &errors.TaskfileVersionCheckError{
			URI:           e.Taskfile.Location,
			SchemaVersion: schemaVersion,
			Message:       fmt.Sprintf(`is greater than the current version of Task (%s)`, currentVersion.String()),
		}
	}

	return nil
}
