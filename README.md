# Chomp

Chomp is a small, dependency-free Go package for parsing command flags and
positionals, routing commands, and rendering readable usage text.

It supports string and bool flags, long and single-short forms, interspersed
flags and positionals, defaults, required flags, `--`, `--help`, and explicit
command routing. Chomp intentionally does not provide shell completion,
environment binding, lifecycle hooks, or command execution.

## Install

```sh
go get github.com/massivemoose/chomp
```

## Quick Start

```go
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/massivemoose/chomp"
)

func main() {
	spec := chomp.New("report").
		String("format",
			chomp.Short('f'),
			chomp.Default("table"),
			chomp.ValueName("name"),
			chomp.Description("output format"),
		).
		Bool("verbose",
			chomp.Short('v'),
			chomp.Description("show extra detail"),
		).
		Positionals(1, 1, "input")

	result, err := spec.ParseCommandLine()
	if errors.Is(err, chomp.ErrHelp) {
		fmt.Print(spec.Usage())
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	fmt.Printf("format=%s verbose=%t input=%s\n",
		result.String("format"),
		result.Bool("verbose"),
		result.Positional(0),
	)
}
```

The spec above accepts commands such as:

```text
report -v --format=json input.csv
report input.csv -f json
report --help
```

Defaults are returned as parsed values, while `Result.IsSet` reports whether
the user explicitly supplied a flag. Required flags are presence-based, so an
explicit empty string or explicit false bool still satisfies `Required()`.

`ParseCommandLine` parses the current process arguments after the executable
name. Use `Parse(args)` when arguments come from another source or when keeping
the parser independent from process-global state is useful.

Long bool flags accept separated values such as `--verbose false`. Short bool
flags use `-v` or `-v=false`; a following argument remains positional.

`Usage()` returns stable unwrapped text. Use `UsageWidth(width)` to wrap flag
descriptions to an explicit width while keeping output independent from the
terminal environment.

## Router

Chomp includes a tiny router for CLIs that want explicit command structs
without adopting a full framework:

```go
type StatusCommand struct{}

func (StatusCommand) Name() string { return "status" }
func (StatusCommand) Summary() string { return "Show project status" }
func (StatusCommand) Run(ctx context.Context, args []string) error {
	// Parse args and run command-specific behavior here.
	return nil
}
func (StatusCommand) Usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage:\n  tool status")
}

router := chomp.NewRouter("tool", "Project tool.", StatusCommand{})
if err := router.Run(context.Background(), os.Args[1:]); err != nil {
	if command, path, ok := chomp.UsageTarget(err); ok {
		chomp.WriteCommandUsage(os.Stdout, command, path)
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
```

Routers dispatch to named commands, support path-aware help such as
`help app inspect`, omit hidden commands from generated usage, and can be
nested either by composing routers or by implementing `Subcommander`. Each
command still owns its own parsing and business logic.

```go
type AppCommand struct{}

func (AppCommand) Name() string { return "app" }
func (AppCommand) Summary() string { return "Manage apps" }
func (AppCommand) Run(context.Context, []string) error {
	return chomp.ErrUsage
}
func (AppCommand) Usage(w io.Writer) {
	chomp.WriteCommandUsage(w, AppCommand{}, []string{"tool"})
}
func (AppCommand) Subcommands() []chomp.Command {
	return []chomp.Command{ListCommand{}, InspectCommand{}}
}
```

Generated parent usage includes the full command path and preserves declaration
order:

```text
Manage apps

Usage:
  tool app <command> [args...]

Commands:
  list     List apps
  inspect  Inspect an app
```

## Scope

Chomp focuses on parser, tiny router, and usage rendering:

- string and bool flags;
- long flags and single-short aliases;
- required flags, descriptions, defaults, and value labels;
- positional arity;
- explicit nested command dispatch;
- stable plain-text usage.

Short clusters, command aliases, inherited flags, shell completion,
env/config binding, color, generated docs, lifecycle hooks, middleware, and
command execution are outside its scope.

## License

Apache-2.0
