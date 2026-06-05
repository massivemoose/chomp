package chomp

import (
	"context"
	"errors"
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

	for _, args := range [][]string{nil, {"help"}, {"--help"}, {"-h"}} {
		err := router.Run(context.Background(), args)
		if !errors.Is(err, ErrUsage) {
			t.Fatalf("expected ErrUsage for %#v, got %v", args, err)
		}
		if _, ok := UsageCommand(err); ok {
			t.Fatalf("expected root usage for %#v, got command usage", args)
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

	usageCommand, ok := UsageCommand(err)
	if !ok {
		t.Fatalf("expected command usage error, got %v", err)
	}
	if usageCommand != leaf {
		t.Fatalf("expected nested usage command %p, got %p", leaf, usageCommand)
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
  ovek <command>

Commands:
  auth         Manage local auth
  status       Show project status
`
	if got := usage.String(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestRouterUsageSkipsHiddenCommands(t *testing.T) {
	router := NewRouter(
		"ovek",
		"Ovek CLI",
		&recordingCommand{name: "run", summary: "Run capsule image"},
		&recordingCommand{name: "deploy", summary: "Legacy deploy", hidden: true},
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
	name    string
	summary string
	hidden  bool
	args    []string
	err     error
}

func (command *recordingCommand) Name() string { return command.name }

func (command *recordingCommand) Summary() string { return command.summary }

func (command *recordingCommand) Hidden() bool { return command.hidden }

func (command *recordingCommand) Run(_ context.Context, args []string) error {
	command.args = append([]string(nil), args...)
	return command.err
}

func (command *recordingCommand) Usage(_ io.Writer) {}
