package chomp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ErrUsage is returned when command usage should be shown.
var ErrUsage = errors.New("usage")

// Command is a runnable CLI command.
type Command interface {
	Name() string
	Summary() string
	Run(context.Context, []string) error
	Usage(io.Writer)
}

// HiddenCommand marks a command as omitted from router usage.
type HiddenCommand interface {
	Hidden() bool
}

// UsageError wraps ErrUsage with the command whose usage should be shown.
type UsageError struct {
	Command Command
}

func (err *UsageError) Error() string {
	return ErrUsage.Error()
}

func (err *UsageError) Unwrap() error {
	return ErrUsage
}

// UsageCommand returns the command whose usage should be shown.
func UsageCommand(err error) (Command, bool) {
	var usageErr *UsageError
	if !errors.As(err, &usageErr) || usageErr.Command == nil {
		return nil, false
	}
	return usageErr.Command, true
}

// Router dispatches arguments to named commands.
type Router struct {
	name     string
	summary  string
	commands map[string]Command
	order    []string
}

// NewRouter creates a command router.
func NewRouter(name string, summary string, commands ...Command) *Router {
	router := &Router{
		name:     name,
		summary:  summary,
		commands: make(map[string]Command, len(commands)),
		order:    make([]string, 0, len(commands)),
	}

	for _, command := range commands {
		validateRouterCommand(command)
		commandName := command.Name()
		if _, exists := router.commands[commandName]; exists {
			panic(fmt.Sprintf("chomp: duplicate command %q", commandName))
		}
		router.commands[commandName] = command
		router.order = append(router.order, commandName)
	}

	sort.Strings(router.order)
	return router
}

// Name returns the router command name.
func (router *Router) Name() string {
	return router.name
}

// Summary returns the router summary.
func (router *Router) Summary() string {
	return router.summary
}

// Run dispatches args to a known command.
func (router *Router) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return ErrUsage
	}
	if args[0] == "-h" || args[0] == "--help" {
		return ErrUsage
	}
	if args[0] == "help" {
		if len(args) == 1 {
			return ErrUsage
		}
		command, ok := router.commands[args[1]]
		if !ok {
			return fmt.Errorf("unknown command %q", args[1])
		}
		return &UsageError{Command: command}
	}

	command, ok := router.commands[args[0]]
	if !ok {
		return fmt.Errorf("unknown command %q", args[0])
	}

	if err := command.Run(ctx, args[1:]); err != nil {
		if _, ok := UsageCommand(err); ok {
			return err
		}
		if errors.Is(err, ErrUsage) || errors.Is(err, ErrHelp) {
			return &UsageError{Command: command}
		}
		return err
	}
	return nil
}

// Usage writes stable plain-text router usage.
func (router *Router) Usage(w io.Writer) {
	if router.summary != "" {
		_, _ = fmt.Fprintf(w, "%s\n\n", router.summary)
	}
	_, _ = fmt.Fprintf(w, "Usage:\n  %s <command>\n\n", router.name)
	_, _ = fmt.Fprintf(w, "Commands:\n")
	for _, commandName := range router.order {
		command := router.commands[commandName]
		if hidden, ok := command.(HiddenCommand); ok && hidden.Hidden() {
			continue
		}
		_, _ = fmt.Fprintf(w, "  %-12s %s\n", command.Name(), command.Summary())
	}
}

func validateRouterCommand(command Command) {
	if command == nil {
		panic("chomp: nil command")
	}
	commandName := command.Name()
	switch {
	case commandName == "":
		panic("chomp: command name cannot be empty")
	case commandName == "help":
		panic(`chomp: reserved command "help"`)
	case strings.ContainsAny(commandName, " \t\r\n"):
		panic(fmt.Sprintf("chomp: command name cannot contain whitespace: %q", commandName))
	}
}
