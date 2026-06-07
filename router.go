package chomp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
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

// Subcommander marks a command as owning nested commands.
type Subcommander interface {
	Subcommands() []Command
}

// UsageGroup describes a generated usage section for child commands.
type UsageGroup struct {
	Key   string
	Title string
}

// UsageGrouper marks a parent command as defining generated usage sections.
type UsageGrouper interface {
	UsageGroups() []UsageGroup
}

// UsageGroupedCommand marks a command as belonging to a generated usage section.
type UsageGroupedCommand interface {
	UsageGroup() string
}

// HiddenCommand marks a command as omitted from router usage.
type HiddenCommand interface {
	Hidden() bool
}

// UsageError wraps ErrUsage with the command whose usage should be shown.
type UsageError struct {
	Command Command
	Path    []string
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

// UsageTarget returns the command and parent path whose usage should be shown.
func UsageTarget(err error) (Command, []string, bool) {
	var usageErr *UsageError
	if !errors.As(err, &usageErr) || usageErr.Command == nil {
		return nil, nil, false
	}
	return usageErr.Command, append([]string(nil), usageErr.Path...), true
}

// Router dispatches arguments to named commands.
type Router struct {
	name     string
	summary  string
	commands []Command
}

// NewRouter creates a command router.
func NewRouter(name string, summary string, commands ...Command) *Router {
	validateCommandName(name)
	validateCommandTree(commands)
	router := &Router{
		name:     name,
		summary:  summary,
		commands: append([]Command(nil), commands...),
	}
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

// Subcommands returns the router's child commands in declaration order.
func (router *Router) Subcommands() []Command {
	return append([]Command(nil), router.commands...)
}

// Run dispatches args to a known command.
func (router *Router) Run(ctx context.Context, args []string) error {
	return dispatch(ctx, router, nil, args)
}

// Usage writes stable plain-text router usage.
func (router *Router) Usage(w io.Writer) {
	WriteCommandUsage(w, router, nil)
}

// WriteCommandUsage writes generated usage for parent commands.
func WriteCommandUsage(w io.Writer, command Command, path []string) {
	children, hasChildren := visibleSubcommands(command)
	if !hasChildren {
		command.Usage(w)
		return
	}

	if summary := command.Summary(); summary != "" {
		_, _ = fmt.Fprintf(w, "%s\n\n", summary)
	}
	_, _ = fmt.Fprintf(w, "Usage:\n  %s <command> [args...]\n\n", fullCommandName(path, command))

	sections := usageSections(command, children)
	width := commandNameWidth(children)
	for index, section := range sections {
		if index > 0 {
			_, _ = fmt.Fprintln(w)
		}
		title := "Commands"
		if len(sections) > 1 && section.key != "" {
			title = section.title
		}
		_, _ = fmt.Fprintf(w, "%s:\n", title)
		for _, child := range section.commands {
			_, _ = fmt.Fprintf(w, "  %-*s  %s\n", width, child.Name(), child.Summary())
		}
	}
}

func dispatch(ctx context.Context, command Command, path []string, args []string) error {
	if len(args) > 0 {
		switch {
		case isHelpArg(args[0]):
			return &UsageError{Command: command, Path: copyPath(path)}
		case args[0] == "help":
			return dispatchHelp(command, path, args[1:])
		}

		if child, ok := findChild(command, args[0]); ok {
			return dispatch(ctx, child, appendPath(path, command.Name()), args[1:])
		}
	}

	if _, ok := command.(*Router); ok {
		if len(args) == 0 {
			return &UsageError{Command: command, Path: copyPath(path)}
		}
		return fmt.Errorf("unknown command %q", args[0])
	}

	if err := command.Run(ctx, args); err != nil {
		if _, _, ok := UsageTarget(err); ok {
			return err
		}
		if errors.Is(err, ErrUsage) || errors.Is(err, ErrHelp) {
			return &UsageError{Command: command, Path: copyPath(path)}
		}
		return err
	}
	return nil
}

func dispatchHelp(command Command, path []string, args []string) error {
	if len(args) == 0 {
		return &UsageError{Command: command, Path: copyPath(path)}
	}

	target := command
	targetPath := copyPath(path)
	for _, name := range args {
		child, ok := findChild(target, name)
		if !ok {
			return fmt.Errorf("unknown command %q", name)
		}
		targetPath = append(targetPath, target.Name())
		target = child
	}
	return &UsageError{Command: target, Path: targetPath}
}

func findChild(command Command, name string) (Command, bool) {
	parent, ok := command.(Subcommander)
	if !ok {
		return nil, false
	}
	for _, child := range parent.Subcommands() {
		if child != nil && child.Name() == name {
			return child, true
		}
	}
	return nil, false
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help"
}

func copyPath(path []string) []string {
	return append([]string(nil), path...)
}

func appendPath(path []string, name string) []string {
	next := make([]string, 0, len(path)+1)
	next = append(next, path...)
	next = append(next, name)
	return next
}

func fullCommandName(path []string, command Command) string {
	parts := appendPath(path, command.Name())
	return strings.Join(parts, " ")
}

func visibleSubcommands(command Command) ([]Command, bool) {
	parent, ok := command.(Subcommander)
	if !ok {
		return nil, false
	}

	children := parent.Subcommands()
	visible := make([]Command, 0, len(children))
	for _, child := range children {
		if child == nil {
			continue
		}
		if hidden, ok := child.(HiddenCommand); ok && hidden.Hidden() {
			continue
		}
		visible = append(visible, child)
	}
	return visible, len(children) > 0 || isRouter(command)
}

func commandNameWidth(commands []Command) int {
	width := 0
	for _, command := range commands {
		if nameWidth := utf8.RuneCountInString(command.Name()); nameWidth > width {
			width = nameWidth
		}
	}
	return width
}

type usageSection struct {
	key      string
	title    string
	commands []Command
}

func usageSections(parent Command, visible []Command) []usageSection {
	groups := usageGroupDefinitions(parent)
	if len(groups) == 0 {
		return []usageSection{{commands: visible}}
	}

	sections := make([]usageSection, 0, len(groups)+1)
	sectionsByKey := make(map[string]int, len(groups)+1)
	defaultListed := false
	for _, group := range groups {
		if group.Key == "" {
			defaultListed = true
			break
		}
	}
	if !defaultListed {
		sectionsByKey[""] = len(sections)
		sections = append(sections, usageSection{})
	}
	for _, group := range groups {
		if group.Key != "" {
			if _, exists := sectionsByKey[group.Key]; exists {
				panic(fmt.Sprintf("chomp: duplicate usage group %q", group.Key))
			}
		} else if _, exists := sectionsByKey[group.Key]; exists {
			continue
		}
		title := group.Title
		if title == "" {
			title = group.Key
		}
		sectionsByKey[group.Key] = len(sections)
		sections = append(sections, usageSection{key: group.Key, title: title})
	}

	for _, child := range allSubcommands(parent) {
		if child == nil {
			continue
		}
		groupKey := usageGroupKey(child)
		sectionIndex, ok := sectionsByKey[groupKey]
		if !ok {
			panic(fmt.Sprintf("chomp: unknown usage group %q for command %q", groupKey, child.Name()))
		}
		if hidden, ok := child.(HiddenCommand); ok && hidden.Hidden() {
			continue
		}
		sections[sectionIndex].commands = append(sections[sectionIndex].commands, child)
	}

	rendered := make([]usageSection, 0, len(sections))
	for _, section := range sections {
		if len(section.commands) > 0 {
			rendered = append(rendered, section)
		}
	}
	if len(rendered) == 0 {
		return []usageSection{{commands: visible}}
	}
	return rendered
}

func usageGroupDefinitions(command Command) []UsageGroup {
	grouper, ok := command.(UsageGrouper)
	if !ok {
		return nil
	}
	return grouper.UsageGroups()
}

func allSubcommands(command Command) []Command {
	parent, ok := command.(Subcommander)
	if !ok {
		return nil
	}
	return parent.Subcommands()
}

func usageGroupKey(command Command) string {
	grouped, ok := command.(UsageGroupedCommand)
	if !ok {
		return ""
	}
	return grouped.UsageGroup()
}

func isRouter(command Command) bool {
	_, ok := command.(*Router)
	return ok
}

func validateCommandTree(commands []Command) {
	seen := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		validateCommand(command)
		commandName := command.Name()
		if _, exists := seen[commandName]; exists {
			panic(fmt.Sprintf("chomp: duplicate command %q", commandName))
		}
		seen[commandName] = struct{}{}

		if parent, ok := command.(Subcommander); ok {
			validateCommandTree(parent.Subcommands())
		}
	}
}

func validateCommand(command Command) {
	if command == nil {
		panic("chomp: nil command")
	}
	validateCommandName(command.Name())
}

func validateCommandName(commandName string) {
	switch {
	case commandName == "":
		panic("chomp: command name cannot be empty")
	case commandName == "help":
		panic(`chomp: reserved command "help"`)
	case strings.ContainsAny(commandName, " \t\r\n"):
		panic(fmt.Sprintf("chomp: command name cannot contain whitespace: %q", commandName))
	}
}
