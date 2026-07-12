package chomp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestRouterDispatchesKnownCommand(t *testing.T) {
	command := &recordingCommand{name: "status", summary: "Show project status"}
	router := NewRouter("ovek", "Ovek CLI", command)

	err := router.Run(context.Background(), []string{"status", "demo-app"})
	if err != nil {
		t.Fatalf("expected dispatch to succeed, got error: %v", err)
	}
	if len(command.args) != 1 || command.args[0] != "demo-app" {
		t.Fatalf("expected forwarded args %q, got %q", []string{"demo-app"}, command.args)
	}
}

func TestRouterReturnsUsageForRootHelp(t *testing.T) {
	router := NewRouter("ovek", "Ovek CLI")

	tests := []struct {
		args     []string
		wantHelp bool
	}{
		{args: nil},
		{args: []string{"help"}, wantHelp: true},
		{args: []string{"--help"}, wantHelp: true},
		{args: []string{"-h"}, wantHelp: true},
	}
	for _, tt := range tests {
		err := router.Run(context.Background(), tt.args)
		if !errors.Is(err, ErrUsage) {
			t.Fatalf("expected ErrUsage for %#v, got %v", tt.args, err)
		}
		if got := errors.Is(err, ErrHelp); got != tt.wantHelp {
			t.Fatalf("expected ErrHelp=%t for %#v, got %v", tt.wantHelp, tt.args, err)
		}
		compatCommand, ok := UsageCommand(err)
		if !ok {
			t.Fatalf("expected root usage command for %#v, got %v", tt.args, err)
		}
		if compatCommand != router {
			t.Fatalf("expected root usage command %p, got %p", router, compatCommand)
		}
		usageCommand, path, ok := UsageTarget(err)
		if !ok {
			t.Fatalf("expected root usage target for %#v, got %v", tt.args, err)
		}
		if usageCommand != router {
			t.Fatalf("expected root usage command %p, got %p", router, usageCommand)
		}
		if len(path) != 0 {
			t.Fatalf("expected empty root path, got %#v", path)
		}
	}
}

func TestRouterReturnsCommandUsageForCommandHelp(t *testing.T) {
	command := &recordingCommand{name: "rm", summary: "Remove runtime", err: ErrUsage}
	router := NewRouter("ovek", "Ovek CLI", command)

	for _, args := range [][]string{{"rm", "--help"}, {"help", "rm"}} {
		err := router.Run(context.Background(), args)
		if !errors.Is(err, ErrUsage) {
			t.Fatalf("expected ErrUsage for %#v, got %v", args, err)
		}
		if !errors.Is(err, ErrHelp) {
			t.Fatalf("expected ErrHelp for %#v, got %v", args, err)
		}

		usageCommand, ok := UsageCommand(err)
		if !ok {
			t.Fatalf("expected command usage error for %#v, got %v", args, err)
		}
		if usageCommand != command {
			t.Fatalf("expected usage command %p, got %p", command, usageCommand)
		}
	}
}

func TestRouterPreservesNestedUsageError(t *testing.T) {
	leaf := &recordingCommand{name: "key", summary: "Manage API keys", err: ErrUsage}
	parent := NewRouter("auth", "Manage auth", leaf)
	router := NewRouter("ovek", "Ovek CLI", parent)

	err := router.Run(context.Background(), []string{"auth", "key", "--help"})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("expected ErrUsage, got %v", err)
	}
	if !errors.Is(err, ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	usageCommand, ok := UsageCommand(err)
	if !ok {
		t.Fatalf("expected command usage error, got %v", err)
	}
	if usageCommand != leaf {
		t.Fatalf("expected nested usage command %p, got %p", leaf, usageCommand)
	}

	usageCommand, path, ok := UsageTarget(err)
	if !ok {
		t.Fatalf("expected nested usage target, got %v", err)
	}
	if usageCommand != leaf {
		t.Fatalf("expected nested usage target command %p, got %p", leaf, usageCommand)
	}
	if got := strings.Join(path, " "); got != "ovek auth" {
		t.Fatalf("expected nested usage path %q, got %q", "ovek auth", got)
	}
}

func TestRouterDispatchesNestedCommand(t *testing.T) {
	inspect := &recordingCommand{name: "inspect", summary: "Inspect app"}
	app := &recordingCommand{name: "app", summary: "Manage apps", subcommands: []Command{inspect}}
	router := NewRouter("ovek", "Ovek CLI", app)

	err := router.Run(context.Background(), []string{"app", "inspect", "api"})
	if err != nil {
		t.Fatalf("expected nested dispatch to succeed, got error: %v", err)
	}
	if got := strings.Join(inspect.args, " "); got != "api" {
		t.Fatalf("expected nested command args %q, got %q", "api", got)
	}
	if len(app.args) != 0 {
		t.Fatalf("expected parent command not to run, got args %#v", app.args)
	}
}

func TestRouterRunsNoArgLeafCommand(t *testing.T) {
	version := &recordingCommand{name: "version", summary: "Print version"}
	router := NewRouter("ovek", "Ovek CLI", version)

	err := router.Run(context.Background(), []string{"version"})
	if err != nil {
		t.Fatalf("expected no-arg leaf command to run, got error: %v", err)
	}
	if version.args == nil {
		t.Fatal("expected no-arg leaf command to record a run")
	}
	if len(version.args) != 0 {
		t.Fatalf("expected no forwarded args, got %#v", version.args)
	}
}

func TestRouterReturnsNestedUsageTargets(t *testing.T) {
	inspect := &recordingCommand{name: "inspect", summary: "Inspect app"}
	app := &recordingCommand{name: "app", summary: "Manage apps", subcommands: []Command{inspect}}
	router := NewRouter("ovek", "Ovek CLI", app)

	tests := []struct {
		name string
		args []string
	}{
		{name: "root help path", args: []string{"help", "app", "inspect"}},
		{name: "nested help path", args: []string{"app", "help", "inspect"}},
		{name: "inline leaf help", args: []string{"app", "inspect", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := router.Run(context.Background(), tt.args)
			if !errors.Is(err, ErrUsage) {
				t.Fatalf("expected ErrUsage, got %v", err)
			}
			command, path, ok := UsageTarget(err)
			if !ok {
				t.Fatalf("expected usage target, got %v", err)
			}
			if command != inspect {
				t.Fatalf("expected inspect usage command %p, got %p", inspect, command)
			}
			if got := strings.Join(path, " "); got != "ovek app" {
				t.Fatalf("expected usage path %q, got %q", "ovek app", got)
			}
		})
	}
}

func TestRouterReturnsParentUsageTargetWhenGroupCommandReturnsUsage(t *testing.T) {
	app := &recordingCommand{name: "app", summary: "Manage apps", err: ErrUsage, subcommands: []Command{
		&recordingCommand{name: "inspect", summary: "Inspect app"},
	}}
	router := NewRouter("ovek", "Ovek CLI", app)

	err := router.Run(context.Background(), []string{"app"})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("expected ErrUsage, got %v", err)
	}
	if errors.Is(err, ErrHelp) {
		t.Fatalf("did not expect ErrHelp, got %v", err)
	}
	command, path, ok := UsageTarget(err)
	if !ok {
		t.Fatalf("expected usage target, got %v", err)
	}
	if command != app {
		t.Fatalf("expected app usage command %p, got %p", app, command)
	}
	if got := strings.Join(path, " "); got != "ovek" {
		t.Fatalf("expected usage path %q, got %q", "ovek", got)
	}
}

func TestRouterPreservesWrappedUsageErrorDetails(t *testing.T) {
	cause := fmt.Errorf("invalid flags: %w", ErrUsage)
	command := &recordingCommand{name: "deploy", err: cause}
	router := NewRouter("tool", "", command)

	err := router.Run(context.Background(), []string{"deploy"})
	if err == nil || err.Error() != "invalid flags: usage" {
		t.Fatalf("expected detailed usage error, got %v", err)
	}
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("expected ErrUsage, got %v", err)
	}
	if errors.Is(err, ErrHelp) {
		t.Fatalf("did not expect ErrHelp, got %v", err)
	}
	usageCommand, _, ok := UsageTarget(err)
	if !ok || usageCommand != command {
		t.Fatalf("expected deploy usage target, got %v", err)
	}
}

func TestRouterPreservesExistingUsageError(t *testing.T) {
	leaf := &recordingCommand{name: "leaf"}
	want := &UsageError{Command: leaf, Path: []string{"custom"}, Cause: ErrHelp}
	command := &recordingCommand{name: "command", err: want}
	router := NewRouter("tool", "", command)

	got := router.Run(context.Background(), []string{"command"})
	if got != want {
		t.Fatalf("expected existing UsageError %p, got %p", want, got)
	}
}

func TestRouterSupportsRunnableParentFallback(t *testing.T) {
	tail := &recordingCommand{name: "tail", summary: "Tail logs"}
	logs := &recordingCommand{name: "logs", summary: "Show logs", subcommands: []Command{tail}}
	router := NewRouter("ovek", "Ovek CLI", logs)

	if err := router.Run(context.Background(), []string{"logs", "api"}); err != nil {
		t.Fatalf("expected runnable parent fallback, got error: %v", err)
	}
	if got := strings.Join(logs.args, " "); got != "api" {
		t.Fatalf("expected logs args %q, got %q", "api", got)
	}

	if err := router.Run(context.Background(), []string{"logs", "tail", "api"}); err != nil {
		t.Fatalf("expected nested child dispatch, got error: %v", err)
	}
	if got := strings.Join(tail.args, " "); got != "api" {
		t.Fatalf("expected tail args %q, got %q", "api", got)
	}
}

func TestRouterReturnsUnknownCommandError(t *testing.T) {
	router := NewRouter("ovek", "Ovek CLI")

	err := router.Run(context.Background(), []string{"wat"})
	if err == nil || err.Error() != `unknown command "wat"` {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func TestRouterUsageListsCommands(t *testing.T) {
	router := NewRouter(
		"ovek",
		"Ovek CLI",
		&recordingCommand{name: "status", summary: "Show project status"},
		&recordingCommand{name: "auth", summary: "Manage local auth"},
	)

	var usage strings.Builder
	router.Usage(&usage)

	const want = `Ovek CLI

Usage:
  ovek <command> [args...]

Commands:
  status  Show project status
  auth    Manage local auth
`
	if got := usage.String(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteCommandUsageRendersNestedPathAndDeclarationOrder(t *testing.T) {
	router := NewRouter(
		"backlot",
		"Backlot CLI",
		NewRouter(
			"autosync",
			"Sync workspace state",
			&recordingCommand{name: "status", summary: "Show sync status"},
			&recordingCommand{name: "enable", summary: "Enable autosync"},
		),
	)

	autosync := router.Subcommands()[0]

	var usage strings.Builder
	WriteCommandUsage(&usage, autosync, []string{"backlot"})

	const want = `Sync workspace state

Usage:
  backlot autosync <command> [args...]

Commands:
  status  Show sync status
  enable  Enable autosync
`
	if got := usage.String(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteCommandUsageGroupsCommands(t *testing.T) {
	router := &groupedRouter{
		Router: NewRouter(
			"tool",
			"Project tool",
			&recordingCommand{name: "status", summary: "Show status"},
			&recordingCommand{name: "deploy", summary: "Deploy an app", usageGroup: "workflow"},
			&recordingCommand{name: "logs", summary: "Show logs", usageGroup: "workflow"},
			&recordingCommand{name: "server", summary: "Manage server", usageGroup: "admin"},
		),
		groups: []UsageGroup{
			{Key: "workflow", Title: "Workflow"},
			{Key: "admin", Title: "Admin"},
		},
	}

	var usage strings.Builder
	router.Usage(&usage)

	const want = `Project tool

Usage:
  tool <command> [args...]

Commands:
  status  Show status

Workflow:
  deploy  Deploy an app
  logs    Show logs

Admin:
  server  Manage server
`
	if got := usage.String(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteCommandUsageCanPositionDefaultGroup(t *testing.T) {
	router := &groupedRouter{
		Router: NewRouter(
			"tool",
			"Project tool",
			&recordingCommand{name: "deploy", summary: "Deploy an app", usageGroup: "workflow"},
			&recordingCommand{name: "status", summary: "Show status"},
		),
		groups: []UsageGroup{
			{Key: "workflow", Title: "Workflow"},
			{},
		},
	}

	var usage strings.Builder
	router.Usage(&usage)

	const want = `Project tool

Usage:
  tool <command> [args...]

Workflow:
  deploy  Deploy an app

Commands:
  status  Show status
`
	if got := usage.String(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteCommandUsageFallsBackToGroupKeyForTitle(t *testing.T) {
	router := &groupedRouter{
		Router: NewRouter(
			"tool",
			"Project tool",
			&recordingCommand{name: "status", summary: "Show status"},
			&recordingCommand{name: "deploy", summary: "Deploy an app", usageGroup: "workflow"},
		),
		groups: []UsageGroup{{Key: "workflow"}},
	}

	var usage strings.Builder
	router.Usage(&usage)

	const want = `Project tool

Usage:
  tool <command> [args...]

Commands:
  status  Show status

workflow:
  deploy  Deploy an app
`
	if got := usage.String(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteCommandUsageSingleNamedGroupUsesCommandsHeading(t *testing.T) {
	router := &groupedRouter{
		Router: NewRouter(
			"tool",
			"Project tool",
			&recordingCommand{name: "deploy", summary: "Deploy an app", usageGroup: "workflow"},
			&recordingCommand{name: "logs", summary: "Show logs", usageGroup: "workflow"},
		),
		groups: []UsageGroup{{Key: "workflow", Title: "Workflow"}},
	}

	var usage strings.Builder
	router.Usage(&usage)

	const want = `Project tool

Usage:
  tool <command> [args...]

Commands:
  deploy  Deploy an app
  logs    Show logs
`
	if got := usage.String(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteCommandUsageSkipsEmptyAndHiddenOnlyGroups(t *testing.T) {
	router := &groupedRouter{
		Router: NewRouter(
			"tool",
			"Project tool",
			&recordingCommand{name: "status", summary: "Show status"},
			&recordingCommand{name: "debug", summary: "Debug internals", hidden: true, usageGroup: "internal"},
		),
		groups: []UsageGroup{
			{Key: "workflow", Title: "Workflow"},
			{Key: "internal", Title: "Internal"},
		},
	}

	var usage strings.Builder
	router.Usage(&usage)

	const want = `Project tool

Usage:
  tool <command> [args...]

Commands:
  status  Show status
`
	if got := usage.String(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestRouterUsageSkipsHiddenCommands(t *testing.T) {
	hidden := &recordingCommand{name: "deploy", summary: "Legacy deploy", hidden: true}
	router := NewRouter(
		"ovek",
		"Ovek CLI",
		&recordingCommand{name: "run", summary: "Run capsule image"},
		hidden,
	)

	var usage strings.Builder
	router.Usage(&usage)

	text := usage.String()
	if !strings.Contains(text, "run") {
		t.Fatalf("expected usage to contain visible command, got %q", text)
	}
	if strings.Contains(text, "deploy") {
		t.Fatalf("expected usage to hide deploy command, got %q", text)
	}

	if err := router.Run(context.Background(), []string{"deploy"}); err != nil {
		t.Fatalf("expected hidden command to dispatch, got %v", err)
	}

	err := router.Run(context.Background(), []string{"help", "deploy"})
	command, _, ok := UsageTarget(err)
	if !ok {
		t.Fatalf("expected direct hidden help target, got %v", err)
	}
	if command != hidden {
		t.Fatalf("expected hidden usage target %p, got %p", hidden, command)
	}
}

func TestNewRouterPanicsForInvalidCommands(t *testing.T) {
	tests := []struct {
		name string
		run  func()
		want string
	}{
		{
			name: "nil command",
			run: func() {
				NewRouter("ovek", "", nil)
			},
			want: "chomp: nil command",
		},
		{
			name: "empty command name",
			run: func() {
				NewRouter("ovek", "", &recordingCommand{})
			},
			want: "chomp: command name cannot be empty",
		},
		{
			name: "duplicate command name",
			run: func() {
				NewRouter("ovek", "", &recordingCommand{name: "status"}, &recordingCommand{name: "status"})
			},
			want: `chomp: duplicate command "status"`,
		},
		{
			name: "nested nil command",
			run: func() {
				NewRouter("ovek", "", &recordingCommand{name: "app", subcommands: []Command{nil}})
			},
			want: "chomp: nil command",
		},
		{
			name: "nested empty command name",
			run: func() {
				NewRouter("ovek", "", &recordingCommand{name: "app", subcommands: []Command{&recordingCommand{}}})
			},
			want: "chomp: command name cannot be empty",
		},
		{
			name: "duplicate nested command name",
			run: func() {
				NewRouter("ovek", "", &recordingCommand{name: "app", subcommands: []Command{
					&recordingCommand{name: "status"},
					&recordingCommand{name: "status"},
				}})
			},
			want: `chomp: duplicate command "status"`,
		},
		{
			name: "reserved help command",
			run: func() {
				NewRouter("ovek", "", &recordingCommand{name: "help"})
			},
			want: `chomp: reserved command "help"`,
		},
		{
			name: "whitespace command name",
			run: func() {
				NewRouter("ovek", "", &recordingCommand{name: "bad name"})
			},
			want: `chomp: command name cannot contain whitespace: "bad name"`,
		},
		{
			name: "invalid root name",
			run: func() {
				NewRouter("bad root", "")
			},
			want: `chomp: command name cannot contain whitespace: "bad root"`,
		},
		{
			name: "duplicate usage group",
			run: func() {
				router := &groupedRouter{
					Router: NewRouter("tool", "", &recordingCommand{name: "deploy", usageGroup: "workflow"}),
					groups: []UsageGroup{
						{Key: "workflow", Title: "Workflow"},
						{Key: "workflow", Title: "Workflows"},
					},
				}
				var usage strings.Builder
				router.Usage(&usage)
			},
			want: `chomp: duplicate usage group "workflow"`,
		},
		{
			name: "unknown usage group",
			run: func() {
				router := &groupedRouter{
					Router: NewRouter("tool", "", &recordingCommand{name: "deploy", usageGroup: "workflow"}),
					groups: []UsageGroup{{Key: "admin", Title: "Admin"}},
				}
				var usage strings.Builder
				router.Usage(&usage)
			},
			want: `chomp: unknown usage group "workflow" for command "deploy"`,
		},
		{
			name: "unknown hidden usage group",
			run: func() {
				router := &groupedRouter{
					Router: NewRouter("tool", "", &recordingCommand{name: "debug", hidden: true, usageGroup: "internal"}),
					groups: []UsageGroup{{Key: "admin", Title: "Admin"}},
				}
				var usage strings.Builder
				router.Usage(&usage)
			},
			want: `chomp: unknown usage group "internal" for command "debug"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					t.Fatal("expected panic")
				}
				if got := recovered.(string); got != tt.want {
					t.Fatalf("expected panic %q, got %q", tt.want, got)
				}
			}()

			tt.run()
		})
	}
}

type recordingCommand struct {
	name        string
	summary     string
	hidden      bool
	args        []string
	err         error
	subcommands []Command
	usageGroup  string
}

func (command *recordingCommand) Name() string { return command.name }

func (command *recordingCommand) Summary() string { return command.summary }

func (command *recordingCommand) Hidden() bool { return command.hidden }

func (command *recordingCommand) UsageGroup() string { return command.usageGroup }

func (command *recordingCommand) Subcommands() []Command {
	return command.subcommands
}

func (command *recordingCommand) Run(_ context.Context, args []string) error {
	command.args = append([]string{}, args...)
	return command.err
}

func (command *recordingCommand) Usage(_ io.Writer) {}

type groupedRouter struct {
	*Router
	groups []UsageGroup
}

func (router *groupedRouter) Usage(w io.Writer) {
	WriteCommandUsage(w, router, nil)
}

func (router *groupedRouter) UsageGroups() []UsageGroup {
	return router.groups
}
