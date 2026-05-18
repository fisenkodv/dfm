// Package main is the dfm CLI entry point.
//
// dfm is a standalone dotfiles manager.
// All logic lives in the app/ package; main.go only wires up flag parsing,
// signal handling, and dispatch.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/go-pkgz/lgr"
	"github.com/jessevdk/go-flags"

	"github.com/bitcldr/dfm/app/cmd"
)

// revision is set at build time via -ldflags "-X main.revision=...".
var revision = "dev"

// Opts holds global flags and subcommands. go-flags populates each subcommand
// field when the corresponding verb is invoked; the selected subcommand's
// Execute method is then called.
type Opts struct {
	BaseDir    string `short:"C" long:"dir" description:"base directory for resolving profiles and sources" default:"."`
	ConfigPath string `short:"c" long:"config" description:"explicit config path (overrides profile name lookup)"`
	Verbose    bool   `long:"verbose" description:"enable verbose (debug) logging"`
	Quiet      bool   `short:"q" long:"quiet" description:"suppress info output (warnings and errors still shown)"`

	Apply      cmd.ApplyCmd      `command:"apply" description:"apply one or more profiles"`
	Diff       cmd.DiffCmd       `command:"diff" description:"show planned changes without writing"`
	Doctor     cmd.DoctorCmd     `command:"doctor" description:"verify installed symlinks still resolve"`
	Status     cmd.StatusCmd     `command:"status" description:"show last applied profiles"`
	List       cmd.ListCmd       `command:"list" description:"list available profiles"`
	Completion cmd.CompletionCmd `command:"completion" description:"output shell completion script"`
}

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var opts Opts
	parser := flags.NewParser(&opts, flags.Default)
	parser.Name = "dfm"
	parser.LongDescription = fmt.Sprintf("dotfiles manager (%s)", revision)

	// Inject shared config into every subcommand before Execute runs.
	parser.CommandHandler = func(command flags.Commander, args []string) error {
		if opts.Verbose && opts.Quiet {
			return fmt.Errorf("--verbose and --quiet are mutually exclusive")
		}
		setupLogger(opts.Verbose, opts.Quiet)

		if c, ok := command.(cmd.ContextSetter); ok {
			c.SetContext(ctx)
		}

		if c, ok := command.(cmd.GlobalsSetter); ok {
			c.SetGlobals(cmd.Globals{
				BaseDir:    opts.BaseDir,
				ConfigPath: opts.ConfigPath,
				Revision:   revision,
			})
		}

		return command.Execute(args)
	}

	if _, err := parser.Parse(); err != nil {
		var fe *flags.Error

		if errors.As(err, &fe) && fe.Type == flags.ErrHelp {
			return 0
		}

		return 1
	}

	return 0
}

func setupLogger(verbose, quiet bool) {
	logOpts := make([]lgr.Option, 0, 2)
	logOpts = append(logOpts, lgr.Format("{{.Message}}"))

	switch {
	case verbose:
		logOpts = []lgr.Option{lgr.Debug, lgr.Msec, lgr.LevelBraces, lgr.StackTraceOnError}
	case quiet:
		logOpts = []lgr.Option{lgr.Out(io.Discard), lgr.Err(io.Discard)}
	}

	colorizer := lgr.Mapper{
		ErrorFunc:  func(s string) string { return color.New(color.FgHiRed).Sprint(s) },
		WarnFunc:   func(s string) string { return color.New(color.FgRed).Sprint(s) },
		InfoFunc:   func(s string) string { return color.New(color.FgYellow).Sprint(s) },
		DebugFunc:  func(s string) string { return color.New(color.FgWhite).Sprint(s) },
		CallerFunc: func(s string) string { return color.New(color.FgBlue).Sprint(s) },
		TimeFunc:   func(s string) string { return color.New(color.FgCyan).Sprint(s) },
	}

	logOpts = append(logOpts, lgr.Map(colorizer))
	lgr.SetupStdLogger(logOpts...)
	lgr.Setup(logOpts...)
}
