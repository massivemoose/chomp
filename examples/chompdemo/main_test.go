package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRootHelpRendersGroupedUsage(t *testing.T) {
	result := runDemo(t, []string{"help"})

	result.wantCode(0)
	result.wantStdoutContains(
		"Chomp demo CLI\n\n",
		"Usage:\n  chompdemo <command> [args...]\n\n",
		"Commands:\n  version  Print demo version\n\n",
		"Manage:\n  app      Manage apps\n  server   Manage server\n\n",
		"Workflow:\n  deploy   Deploy an app\n  logs     Show logs\n",
	)
	result.wantStdoutNotContains("debug", "Internal")
	result.wantStderr("")
}

func TestRootFlagHelpExitsSuccessfully(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}} {
		result := runDemo(t, args)

		result.wantCode(0)
		result.wantStdoutContains("Usage:\n  chompdemo <command> [args...]\n\n")
		result.wantStderr("")
	}
}

func TestNestedParentHelpRendersFullPath(t *testing.T) {
	result := runDemo(t, []string{"app"})

	result.wantCode(2)
	result.wantStdout(`Manage apps

Usage:
  chompdemo app <command> [args...]

Commands:
  list     List apps
  inspect  Inspect app
`)
	result.wantStderr("")
}

func TestLeafHelpRendersParserUsage(t *testing.T) {
	result := runDemo(t, []string{"app", "inspect", "--help"})

	result.wantCode(0)
	result.wantStdoutContains(
		"Usage: chompdemo app inspect [flags] <app>\n\n",
		"--verbose",
		"show extra inspect details",
		"-h, --help",
	)
	result.wantStderr("")
}

func TestAppListParsesFormatFlag(t *testing.T) {
	result := runDemo(t, []string{"app", "list", "--format", "json"})

	result.wantCode(0)
	result.wantStdout(`command=app list
format=json
`)
	result.wantStderr("")
}

func TestDeployParsesTypedFlagsAndPositionals(t *testing.T) {
	result := runDemo(t, []string{
		"deploy", "api",
		"--env", "prod",
		"--image", "registry/app:v1",
		"--replicas", "3",
		"--timeout", "30s",
		"--include", "config",
		"--include", "secrets",
		"--dry-run",
	})

	result.wantCode(0)
	result.wantStdout(`command=deploy
app=api
env=prod
image=registry/app:v1
replicas=3
timeout=30s
include=config,secrets
dry-run=true
`)
	result.wantStderr("")
}

func TestLogsRunsParentCommand(t *testing.T) {
	result := runDemo(t, []string{"logs", "api"})

	result.wantCode(0)
	result.wantStdout(`command=logs
app=api
follow=false
`)
	result.wantStderr("")
}

func TestLogsTailDispatchesToChild(t *testing.T) {
	result := runDemo(t, []string{"logs", "tail", "api", "--follow"})

	result.wantCode(0)
	result.wantStdout(`command=logs tail
app=api
follow=true
`)
	result.wantStderr("")
}

func TestHiddenDebugStillDispatches(t *testing.T) {
	result := runDemo(t, []string{"debug"})

	result.wantCode(0)
	result.wantStdout("command=debug\n")
	result.wantStderr("")
}

func TestUnknownRootCommandReturnsError(t *testing.T) {
	result := runDemo(t, []string{"wat"})

	result.wantCode(1)
	result.wantStdout("")
	result.wantStderr("unknown command \"wat\"\n")
}

func TestParserErrorReturnsUsage(t *testing.T) {
	result := runDemo(t, []string{"deploy", "api"})

	result.wantCode(2)
	result.wantStdoutContains(
		"Usage: chompdemo deploy [flags] <app>\n\n",
		"--image <ref>",
		"--replicas <int>",
		"--timeout <duration>",
		"--include <value>",
		"-h, --help",
	)
	result.wantStderr("")
}

func TestProcessHelpExitsSuccessfully(t *testing.T) {
	result := runProcess(t, "help")

	result.wantCode(0)
	result.wantStdoutContains(
		"Chomp demo CLI\n\n",
		"Usage:\n  chompdemo <command> [args...]\n\n",
	)
	result.wantStderr("")
}

func TestProcessGroupOnlyUsageExitsWithGoRunError(t *testing.T) {
	result := runProcess(t, "app")

	result.wantCode(1)
	result.wantStdoutContains("Usage:\n  chompdemo app <command> [args...]\n\n")
	result.wantStderrContains("exit status 2")
}

func TestProcessUnknownCommandWritesStderr(t *testing.T) {
	result := runProcess(t, "wat")

	result.wantCode(1)
	result.wantStdout("")
	result.wantStderrContains(`unknown command "wat"`, "exit status 1")
}

type demoResult struct {
	t      *testing.T
	code   int
	stdout string
	stderr string
}

func runDemo(t *testing.T, args []string) demoResult {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return demoResult{
		t:      t,
		code:   code,
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
}

func runProcess(t *testing.T, args ...string) demoResult {
	t.Helper()

	commandArgs := append([]string{"run", "."}, args...)
	cmd := exec.Command("go", commandArgs...)
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "GOCACHE="+t.TempDir())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	code := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("failed to run go command: %v", err)
		}
		code = exitErr.ExitCode()
	}

	return demoResult{
		t:      t,
		code:   code,
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
}

func (result demoResult) wantCode(want int) {
	result.t.Helper()
	if result.code != want {
		result.t.Fatalf("expected exit code %d, got %d; stdout=%q stderr=%q", want, result.code, result.stdout, result.stderr)
	}
}

func (result demoResult) wantStdout(want string) {
	result.t.Helper()
	if result.stdout != want {
		result.t.Fatalf("unexpected stdout:\n%s\nwant:\n%s", result.stdout, want)
	}
}

func (result demoResult) wantStdoutContains(wants ...string) {
	result.t.Helper()
	for _, want := range wants {
		if !strings.Contains(result.stdout, want) {
			result.t.Fatalf("expected stdout to contain %q, got:\n%s", want, result.stdout)
		}
	}
}

func (result demoResult) wantStdoutNotContains(wants ...string) {
	result.t.Helper()
	for _, want := range wants {
		if strings.Contains(result.stdout, want) {
			result.t.Fatalf("expected stdout not to contain %q, got:\n%s", want, result.stdout)
		}
	}
}

func (result demoResult) wantStderrContains(wants ...string) {
	result.t.Helper()
	for _, want := range wants {
		if !strings.Contains(result.stderr, want) {
			result.t.Fatalf("expected stderr to contain %q, got:\n%s", want, result.stderr)
		}
	}
}

func (result demoResult) wantStderr(want string) {
	result.t.Helper()
	if result.stderr != want {
		result.t.Fatalf("unexpected stderr %q, want %q", result.stderr, want)
	}
}
