# Chomp

Chomp is a small, dependency-free Go package for parsing command flags and
positionals and rendering readable usage text.

It supports string and bool flags, long and single-short forms, interspersed
flags and positionals, defaults, required flags, `--`, and `--help`. Chomp
intentionally does not provide command routing, shell completion, environment
binding, or command execution.

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

## Scope

Chomp v0.1 focuses on parser and usage rendering:

- string and bool flags;
- long flags and single-short aliases;
- required flags, descriptions, defaults, and value labels;
- positional arity;
- stable plain-text usage.

Command routing, short clusters, aliases, shell completion, env/config
binding, color, generated docs, and command execution are outside its scope.

## License

Apache-2.0
