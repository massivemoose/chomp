package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/massivemoose/chomp"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	router := newRootRouter(stdout)
	if err := router.Run(context.Background(), args); err != nil {
		if command, path, ok := chomp.UsageTarget(err); ok {
			chomp.WriteCommandUsage(stdout, command, path)
			if errors.Is(err, chomp.ErrHelp) {
				return 0
			}
			return 2
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

type rootRouter struct {
	*chomp.Router
}

func newRootRouter(stdout io.Writer) *rootRouter {
	return &rootRouter{
		Router: chomp.NewRouter(
			"chompdemo",
			"Chomp demo CLI",
			versionCommand{stdout: stdout},
			appCommand{stdout: stdout},
			serverCommand{stdout: stdout},
			deployCommand{stdout: stdout},
			logsCommand{stdout: stdout},
			debugCommand{stdout: stdout},
		),
	}
}

func (router *rootRouter) Usage(w io.Writer) {
	chomp.WriteCommandUsage(w, router, nil)
}

func (router *rootRouter) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return &chomp.UsageError{Command: router}
	}
	if args[0] == "-h" || args[0] == "--help" || (args[0] == "help" && len(args) == 1) {
		return &chomp.UsageError{Command: router, Cause: chomp.ErrHelp}
	}
	return router.Router.Run(ctx, args)
}

func (router *rootRouter) UsageGroups() []chomp.UsageGroup {
	return []chomp.UsageGroup{
		{Key: "manage", Title: "Manage"},
		{Key: "workflow", Title: "Workflow"},
		{Key: "internal", Title: "Internal"},
	}
}

type versionCommand struct {
	stdout io.Writer
}

func (versionCommand) Name() string    { return "version" }
func (versionCommand) Summary() string { return "Print demo version" }

func (command versionCommand) Run(_ context.Context, args []string) error {
	if _, err := versionSpec().Parse(args); err != nil {
		return chomp.ErrUsage
	}
	fmt.Fprintln(command.stdout, "command=version")
	fmt.Fprintln(command.stdout, "version=0.0.0-dev")
	return nil
}

func (versionCommand) Usage(w io.Writer) {
	fmt.Fprint(w, versionSpec().Usage())
}

func versionSpec() *chomp.Spec {
	return chomp.New("chompdemo version").
		Positionals(0, 0)
}

type appCommand struct {
	stdout io.Writer
}

func (appCommand) Name() string       { return "app" }
func (appCommand) Summary() string    { return "Manage apps" }
func (appCommand) UsageGroup() string { return "manage" }

func (appCommand) Run(context.Context, []string) error {
	return chomp.ErrUsage
}

func (appCommand) Usage(w io.Writer) {
	chomp.WriteCommandUsage(w, appCommand{}, []string{"chompdemo"})
}

func (command appCommand) Subcommands() []chomp.Command {
	return []chomp.Command{
		appListCommand{stdout: command.stdout},
		appInspectCommand{stdout: command.stdout},
	}
}

type appListCommand struct {
	stdout io.Writer
}

func (appListCommand) Name() string    { return "list" }
func (appListCommand) Summary() string { return "List apps" }

func (command appListCommand) Run(_ context.Context, args []string) error {
	result, err := appListSpec().Parse(args)
	if err != nil {
		return chomp.ErrUsage
	}
	fmt.Fprintln(command.stdout, "command=app list")
	fmt.Fprintf(command.stdout, "format=%s\n", result.String("format"))
	return nil
}

func (appListCommand) Usage(w io.Writer) {
	fmt.Fprint(w, appListSpec().Usage())
}

func appListSpec() *chomp.Spec {
	return chomp.New("chompdemo app list").
		String("format",
			chomp.Default("table"),
			chomp.ValueName("name"),
			chomp.Description("output format name"),
			chomp.OneOf("table", "json"),
		).
		Positionals(0, 0)
}

type appInspectCommand struct {
	stdout io.Writer
}

func (appInspectCommand) Name() string    { return "inspect" }
func (appInspectCommand) Summary() string { return "Inspect app" }

func (command appInspectCommand) Run(_ context.Context, args []string) error {
	result, err := appInspectSpec().Parse(args)
	if err != nil {
		return chomp.ErrUsage
	}
	fmt.Fprintln(command.stdout, "command=app inspect")
	fmt.Fprintf(command.stdout, "app=%s\n", result.Positional(0))
	fmt.Fprintf(command.stdout, "verbose=%t\n", result.Bool("verbose"))
	return nil
}

func (appInspectCommand) Usage(w io.Writer) {
	fmt.Fprint(w, appInspectSpec().Usage())
}

func appInspectSpec() *chomp.Spec {
	return chomp.New("chompdemo app inspect").
		Bool("verbose", chomp.Description("show extra inspect details")).
		Positionals(1, 1, "app")
}

type serverCommand struct {
	stdout io.Writer
}

func (serverCommand) Name() string       { return "server" }
func (serverCommand) Summary() string    { return "Manage server" }
func (serverCommand) UsageGroup() string { return "manage" }

func (serverCommand) Run(context.Context, []string) error {
	return chomp.ErrUsage
}

func (serverCommand) Usage(w io.Writer) {
	chomp.WriteCommandUsage(w, serverCommand{}, []string{"chompdemo"})
}

func (command serverCommand) Subcommands() []chomp.Command {
	return []chomp.Command{serverStatusCommand{stdout: command.stdout}}
}

type serverStatusCommand struct {
	stdout io.Writer
}

func (serverStatusCommand) Name() string    { return "status" }
func (serverStatusCommand) Summary() string { return "Show server status" }

func (command serverStatusCommand) Run(_ context.Context, args []string) error {
	if _, err := serverStatusSpec().Parse(args); err != nil {
		return chomp.ErrUsage
	}
	fmt.Fprintln(command.stdout, "command=server status")
	fmt.Fprintln(command.stdout, "status=ready")
	return nil
}

func (serverStatusCommand) Usage(w io.Writer) {
	fmt.Fprint(w, serverStatusSpec().Usage())
}

func serverStatusSpec() *chomp.Spec {
	return chomp.New("chompdemo server status").
		Positionals(0, 0)
}

type deployCommand struct {
	stdout io.Writer
}

func (deployCommand) Name() string       { return "deploy" }
func (deployCommand) Summary() string    { return "Deploy an app" }
func (deployCommand) UsageGroup() string { return "workflow" }

func (command deployCommand) Run(_ context.Context, args []string) error {
	result, err := deploySpec().Parse(args)
	if err != nil {
		return chomp.ErrUsage
	}
	fmt.Fprintln(command.stdout, "command=deploy")
	fmt.Fprintf(command.stdout, "app=%s\n", result.Positional(0))
	fmt.Fprintf(command.stdout, "env=%s\n", result.String("env"))
	fmt.Fprintf(command.stdout, "image=%s\n", result.String("image"))
	fmt.Fprintf(command.stdout, "replicas=%d\n", result.Int("replicas"))
	fmt.Fprintf(command.stdout, "timeout=%s\n", result.Duration("timeout"))
	fmt.Fprintf(command.stdout, "include=%s\n", strings.Join(result.Strings("include"), ","))
	fmt.Fprintf(command.stdout, "dry-run=%t\n", result.Bool("dry-run"))
	return nil
}

func (deployCommand) Usage(w io.Writer) {
	fmt.Fprint(w, deploySpec().Usage())
}

func deploySpec() *chomp.Spec {
	return chomp.New("chompdemo deploy").
		String("env",
			chomp.Default("dev"),
			chomp.ValueName("name"),
			chomp.Description("target environment"),
			chomp.OneOf("dev", "staging", "prod"),
		).
		String("image",
			chomp.Required(),
			chomp.ValueName("ref"),
			chomp.Description("image reference to deploy"),
		).
		Int("replicas",
			chomp.Default("1"),
			chomp.Description("replica count"),
		).
		Duration("timeout",
			chomp.Default("2m"),
			chomp.Description("deployment timeout"),
		).
		Strings("include",
			chomp.Description("path to include"),
		).
		Bool("dry-run", chomp.Description("print planned work without deploying")).
		Positionals(1, 1, "app")
}

type logsCommand struct {
	stdout io.Writer
}

func (logsCommand) Name() string       { return "logs" }
func (logsCommand) Summary() string    { return "Show logs" }
func (logsCommand) UsageGroup() string { return "workflow" }

func (command logsCommand) Run(_ context.Context, args []string) error {
	result, err := logsSpec("chompdemo logs").Parse(args)
	if err != nil {
		return chomp.ErrUsage
	}
	fmt.Fprintln(command.stdout, "command=logs")
	fmt.Fprintf(command.stdout, "app=%s\n", result.Positional(0))
	fmt.Fprintf(command.stdout, "follow=%t\n", result.Bool("follow"))
	return nil
}

func (logsCommand) Usage(w io.Writer) {
	fmt.Fprint(w, logsSpec("chompdemo logs").Usage())
}

func (command logsCommand) Subcommands() []chomp.Command {
	return []chomp.Command{logsTailCommand{stdout: command.stdout}}
}

type logsTailCommand struct {
	stdout io.Writer
}

func (logsTailCommand) Name() string    { return "tail" }
func (logsTailCommand) Summary() string { return "Tail logs" }

func (command logsTailCommand) Run(_ context.Context, args []string) error {
	result, err := logsSpec("chompdemo logs tail").Parse(args)
	if err != nil {
		return chomp.ErrUsage
	}
	fmt.Fprintln(command.stdout, "command=logs tail")
	fmt.Fprintf(command.stdout, "app=%s\n", result.Positional(0))
	fmt.Fprintf(command.stdout, "follow=%t\n", result.Bool("follow"))
	return nil
}

func (logsTailCommand) Usage(w io.Writer) {
	fmt.Fprint(w, logsSpec("chompdemo logs tail").Usage())
}

func logsSpec(name string) *chomp.Spec {
	return chomp.New(name).
		Bool("follow",
			chomp.Short('f'),
			chomp.Description("follow log output"),
		).
		Positionals(1, 1, "app")
}

type debugCommand struct {
	stdout io.Writer
}

func (debugCommand) Name() string       { return "debug" }
func (debugCommand) Summary() string    { return "Debug internals" }
func (debugCommand) Hidden() bool       { return true }
func (debugCommand) UsageGroup() string { return "internal" }

func (command debugCommand) Run(_ context.Context, args []string) error {
	if _, err := debugSpec().Parse(args); err != nil {
		return chomp.ErrUsage
	}
	fmt.Fprintln(command.stdout, "command=debug")
	return nil
}

func (debugCommand) Usage(w io.Writer) {
	fmt.Fprint(w, debugSpec().Usage())
}

func debugSpec() *chomp.Spec {
	return chomp.New("chompdemo debug").
		Positionals(0, 0)
}
