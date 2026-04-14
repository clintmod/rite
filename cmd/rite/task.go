package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/pflag"

	"github.com/clintmod/rite/args"
	"github.com/clintmod/rite/errors"
	"github.com/clintmod/rite/experiments"
	"github.com/clintmod/rite/internal/filepathext"
	"github.com/clintmod/rite/internal/flags"
	"github.com/clintmod/rite/internal/logger"
	task "github.com/clintmod/rite/internal/task"
	"github.com/clintmod/rite/internal/version"
	"github.com/clintmod/rite/taskfile/ast"
)

func main() {
	if err := run(); err != nil {
		l := &logger.Logger{
			Stdout:  os.Stdout,
			Stderr:  os.Stderr,
			Verbose: flags.Verbose,
			Color:   flags.Color,
		}
		if err, ok := err.(*errors.TaskRunError); ok && flags.ExitCode {
			emitCIErrorAnnotation(err)
			l.Errf(logger.Red, "%v\n", err)
			os.Exit(err.TaskExitCode())
		}
		if err, ok := err.(errors.TaskError); ok {
			emitCIErrorAnnotation(err)
			l.Errf(logger.Red, "%v\n", err)
			os.Exit(err.Code())
		}
		emitCIErrorAnnotation(err)
		l.Errf(logger.Red, "%v\n", err)
		os.Exit(errors.CodeUnknown)
	}
	os.Exit(errors.CodeOk)
}

// emitCIErrorAnnotation emits an error annotation for supported CI providers.
func emitCIErrorAnnotation(err error) {
	if isGA, _ := strconv.ParseBool(os.Getenv("GITHUB_ACTIONS")); !isGA {
		return
	}
	if e, ok := err.(*errors.TaskRunError); ok {
		fmt.Fprintf(os.Stdout, "::error title=Task '%s' failed::%v\n", e.TaskName, e.Err)
		return
	}
	fmt.Fprintf(os.Stdout, "::error title=Task failed::%v\n", err)
}

func run() error {
	log := &logger.Logger{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Verbose: flags.Verbose,
		Color:   flags.Color,
	}

	if err := flags.Validate(); err != nil {
		return err
	}

	if err := experiments.Validate(); err != nil {
		log.Warnf("%s\n", err.Error())
	}

	if flags.Version {
		fmt.Println(version.GetVersionWithBuildInfo())
		return nil
	}

	if flags.Help {
		pflag.Usage()
		return nil
	}

	if flags.Experiments {
		return log.PrintExperiments()
	}

	// Subcommand form: `rite migrate [path]`. Shadows any task named
	// "migrate"; callers who need the token as a task name can still use
	// the `--migrate` flag form, which stays as an alias.
	positional, _, err := args.Get()
	if err != nil {
		return err
	}
	if path, ok := migrateSubcommand(positional); ok {
		return runMigrate(log, path)
	}

	if flags.Migrate {
		src := ""
		if len(positional) > 0 {
			src = positional[0]
		}
		return runMigrate(log, src)
	}

	if flags.Init {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		args, _, err := args.Get()
		if err != nil {
			return err
		}
		path := wd
		if len(args) > 0 {
			name := args[0]
			if filepathext.IsExtOnly(name) {
				name = filepathext.SmartJoin(filepath.Dir(name), "Ritefile"+filepath.Ext(name))
			}
			path = filepathext.SmartJoin(wd, name)
		}
		finalPath, err := task.InitRitefile(path)
		if err != nil {
			return err
		}
		if !flags.Silent {
			if flags.Verbose {
				log.Outf(logger.Default, "%s\n", task.DefaultRitefile)
			}
			log.Outf(logger.Green, "Ritefile created: %s\n", filepathext.TryAbsToRel(finalPath))
		}
		return nil
	}

	if flags.Completion != "" {
		script, err := task.Completion(flags.Completion)
		if err != nil {
			return err
		}
		fmt.Println(script)
		return nil
	}

	e := task.NewExecutor(
		flags.WithFlags(),
		task.WithVersionCheck(true),
	)
	if err := e.Setup(); err != nil {
		return err
	}

	listOptions := task.NewListOptions(
		flags.List,
		flags.ListAll,
		flags.ListJson,
		flags.NoStatus,
		flags.Nested,
	)
	if listOptions.ShouldListTasks() {
		if flags.Silent {
			return e.ListTaskNames(flags.ListAll)
		}
		foundTasks, err := e.ListTasks(listOptions)
		if err != nil {
			return err
		}
		if !foundTasks {
			os.Exit(errors.CodeUnknown)
		}
		return nil
	}

	// Parse the remaining arguments
	cliArgsPreDash, cliArgsPostDash, err := args.Get()
	if err != nil {
		return err
	}
	calls, globals := args.Parse(cliArgsPreDash...)

	// If there are no calls, run the default task instead
	if len(calls) == 0 {
		calls = append(calls, &task.Call{Task: "default"})
	}

	// Merge CLI variables first (e.g. FOO=bar) so they take priority over Ritefile defaults
	e.Ritefile.Vars.Merge(globals, nil)

	// Then ReverseMerge special variables so they're available for templating.
	// CLI_* specials are rite-internal — marked export: false so they don't
	// leak into cmd shell environments alongside user vars. Phase 4 wave 3
	// auto-exports vars by default; without this opt-out, a nested
	// `rite` invocation (or any test that inherits os.Environ and then
	// passes its own CLI_ARGS via call.Vars) would see the outer CLI_ARGS=""
	// win at SPEC tier 1, defeating the test's intent.
	cliArgsPostDashQuoted, err := args.ToQuotedString(cliArgsPostDash)
	if err != nil {
		return err
	}
	noExport := false
	specialVars := ast.NewVars()
	specialVars.Set("CLI_ARGS", ast.Var{Value: cliArgsPostDashQuoted, Export: &noExport})
	specialVars.Set("CLI_ARGS_LIST", ast.Var{Value: cliArgsPostDash, Export: &noExport})
	specialVars.Set("CLI_FORCE", ast.Var{Value: flags.Force || flags.ForceAll, Export: &noExport})
	specialVars.Set("CLI_SILENT", ast.Var{Value: flags.Silent, Export: &noExport})
	specialVars.Set("CLI_VERBOSE", ast.Var{Value: flags.Verbose, Export: &noExport})
	specialVars.Set("CLI_ASSUME_YES", ast.Var{Value: flags.AssumeYes, Export: &noExport})
	e.Ritefile.Vars.ReverseMerge(specialVars, nil)
	ctx := context.Background()
	if !flags.Watch {
		var stop context.CancelFunc
		ctx, stop = e.InterceptInterruptSignals(ctx)
		defer stop()
	}

	if flags.Status {
		return e.Status(ctx, calls...)
	}

	return e.Run(ctx, calls...)
}

// migrateSubcommand reports whether the first positional arg is the literal
// "migrate" token, meaning the user invoked `rite migrate [path]`. The
// returned path is the second positional arg or "" if the user relied on
// autodetection. Extra args past the path are ignored (the existing
// --migrate flag form has the same behavior).
func migrateSubcommand(positional []string) (string, bool) {
	if len(positional) == 0 || positional[0] != "migrate" {
		return "", false
	}
	if len(positional) > 1 {
		return positional[1], true
	}
	return "", true
}

func runMigrate(log *logger.Logger, src string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if src == "" {
		// Autodetect: first existing Ritefile* in cwd.
		for _, name := range []string{"Ritefile.yml", "Ritefile.yaml", "Ritefile.dist.yml", "Ritefile.dist.yaml"} {
			p := filepathext.SmartJoin(wd, name)
			if _, err := os.Stat(p); err == nil {
				src = p
				break
			}
		}
		if src == "" {
			return fmt.Errorf("rite: no Ritefile found in %s; pass a path as the first argument", wd)
		}
	}
	if !filepath.IsAbs(src) {
		src = filepathext.SmartJoin(wd, src)
	}
	dst, err := task.Migrate(src, os.Stderr)
	if err != nil {
		return err
	}
	log.Outf(logger.Green, "Ritefile written: %s\n", filepathext.TryAbsToRel(dst))
	return nil
}
