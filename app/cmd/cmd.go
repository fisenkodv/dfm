// Package cmd defines dfm's CLI subcommands.
//
// Each subcommand is a struct implementing go-flags' Commander interface
// (Execute). Global configuration is injected via the Globals and Context
// setters before Execute runs, so subcommand structs remain free of
// boilerplate for shared flags.
package cmd

import (
	"context"

	"github.com/bitcldr/dfm/app/iostreams"
)

// Globals holds configuration shared across all subcommands, derived from
// top-level flags in main.Opts.
type Globals struct {
	BaseDir    string
	ConfigPath string
	Revision   string
}

// GlobalsSetter is implemented by subcommands that accept injected global
// configuration. The parser's CommandHandler calls SetGlobals before Execute.
type GlobalsSetter interface {
	SetGlobals(g Globals)
}

// ContextSetter is implemented by subcommands that accept an injected
// root context (wired to signal handling in main).
type ContextSetter interface {
	SetContext(ctx context.Context)
}

// IOSetter is implemented by subcommands that accept an injected IOStreams.
type IOSetter interface {
	SetIO(ios *iostreams.IOStreams)
}

// base embeds into every subcommand struct to satisfy the setters without
// duplicating fields.
type base struct {
	ctx     context.Context
	globals Globals
	ios     *iostreams.IOStreams
}

func (b *base) SetContext(ctx context.Context)        { b.ctx = ctx }
func (b *base) SetGlobals(g Globals)                  { b.globals = g }
func (b *base) SetIO(ios *iostreams.IOStreams)         { b.ios = ios }

// Context returns the subcommand's root context, defaulting to Background
// if the parser did not inject one (e.g. in tests).
func (b *base) Context() context.Context {
	if b.ctx == nil {
		return context.Background()
	}
	return b.ctx
}

// Globals returns the injected global configuration.
func (b *base) Globals() Globals { return b.globals }

// IO returns the injected IOStreams, falling back to real os streams if not set.
func (b *base) IO() *iostreams.IOStreams {
	if b.ios == nil {
		return iostreams.New()
	}
	return b.ios
}
