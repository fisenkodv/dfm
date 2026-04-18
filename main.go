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
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jessevdk/go-flags"

	"github.com/fisenkodv/dfm/app/cmd"
)

// revision is set at build time via -ldflags "-X main.revision=...".
var revision = "dev"

// Opts holds global flags and subcommands. go-flags populates each subcommand
// field when the corresponding verb is invoked; the selected subcommand's
// Execute method is then called.
type Opts struct {
	BaseDir    string `short:"C" long:"dir" description:"base directory for resolving profiles and sources" default:"."`
	ConfigPath string `short:"c" long:"config" description:"explicit config path (overrides profile name lookup)"`
	Debug      bool   `long:"dbg" description:"enable debug logging"`
	Quiet      bool   `short:"q" long:"quiet" description:"suppress non-error output"`

	Apply  cmd.ApplyCmd  `command:"apply" description:"apply one or more profiles"`
	Diff   cmd.DiffCmd   `command:"diff" description:"show planned changes without writing"`
	Doctor cmd.DoctorCmd `command:"doctor" description:"verify installed symlinks still resolve"`
	Status cmd.StatusCmd `command:"status" description:"show last applied profiles"`
	List   cmd.ListCmd   `command:"list" description:"list available profiles"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var opts Opts
	parser := flags.NewParser(&opts, flags.Default)
	parser.Name = "dfm"
	parser.LongDescription = fmt.Sprintf("dotfiles manager (rev %s)", revision)

	// Inject shared config into every subcommand before Execute runs.
	parser.CommandHandler = func(command flags.Commander, args []string) error {
		setupLogger(opts.Debug, opts.Quiet)
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
			os.Exit(0)
		}
		os.Exit(1)
	}
}

func setupLogger(debug, quiet bool) {
	level := slog.LevelInfo
	switch {
	case debug:
		level = slog.LevelDebug
	case quiet:
		level = slog.LevelWarn
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}
