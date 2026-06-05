package chomp

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// ErrHelp is returned when help was requested.
var ErrHelp = errors.New("help")

// FlagOption configures a string or bool flag.
type FlagOption func(*flagSpec)

// Required marks a flag as required.
func Required() FlagOption {
	return func(flag *flagSpec) {
		flag.required = true
	}
}

// Short assigns a single-rune short name to a flag.
func Short(name rune) FlagOption {
	return func(flag *flagSpec) {
		flag.short = name
	}
}

// Description sets the flag's usage description.
func Description(description string) FlagOption {
	return func(flag *flagSpec) {
		flag.description = description
	}
}

// Default sets the flag's parsed default value.
func Default(value string) FlagOption {
	return func(flag *flagSpec) {
		flag.defaultValue = value
		flag.hasDefault = true
	}
}

// ValueName sets the string flag's value label in usage text.
func ValueName(name string) FlagOption {
	return func(flag *flagSpec) {
		flag.valueName = valueLabel(name)
	}
}

// Spec defines a command's accepted flags and positionals.
type Spec struct {
	command         string
	displayName     string
	flags           map[string]*flagSpec
	shorts          map[rune]*flagSpec
	flagOrder       []*flagSpec
	minPositionals  int
	maxPositionals  int
	positionalNames []string
	definitionErrs  []error
}

type flagSpec struct {
	name         string
	short        rune
	kind         flagKind
	required     bool
	description  string
	defaultValue string
	hasDefault   bool
	valueName    string
}

type flagKind int

const (
	flagKindString flagKind = iota
	flagKindBool
)

// Result contains parsed flag and positional values.
type Result struct {
	strings     map[string]string
	bools       map[string]bool
	seen        map[string]bool
	flagOrder   []string
	positionals []string
}

// New creates a command specification.
func New(command ...string) *Spec {
	return &Spec{
		command:        commandPath(command...),
		flags:          make(map[string]*flagSpec),
		shorts:         make(map[rune]*flagSpec),
		maxPositionals: -1,
	}
}

// DisplayName overrides the normalized command name used in usage and errors.
func (spec *Spec) DisplayName(name string) *Spec {
	spec.displayName = commandPath(name)
	return spec
}

// String declares a string flag.
func (spec *Spec) String(name string, options ...FlagOption) *Spec {
	return spec.flag(name, flagKindString, options...)
}

// Bool declares a bool flag.
func (spec *Spec) Bool(name string, options ...FlagOption) *Spec {
	return spec.flag(name, flagKindBool, options...)
}

// Positionals declares the accepted positional count and usage names.
func (spec *Spec) Positionals(min, max int, names ...string) *Spec {
	spec.minPositionals = min
	spec.maxPositionals = max
	spec.positionalNames = append([]string(nil), names...)
	return spec
}

// Parse parses command arguments according to the specification.
func (spec *Spec) Parse(args []string) (Result, error) {
	if err := spec.Validate(); err != nil {
		return Result{}, err
	}

	result := Result{
		strings: make(map[string]string),
		bools:   make(map[string]bool),
		seen:    make(map[string]bool),
	}
	for _, flag := range spec.flagOrder {
		if !flag.hasDefault {
			continue
		}
		switch flag.kind {
		case flagKindString:
			result.strings[flag.name] = flag.defaultValue
		case flagKindBool:
			result.bools[flag.name], _ = maybeBoolValue(flag.defaultValue)
		}
	}

	parseFlags := true
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if parseFlags && arg == "--" {
			parseFlags = false
			continue
		}
		if parseFlags && (arg == "-h" || arg == "--help") {
			return Result{}, ErrHelp
		}
		if parseFlags && strings.HasPrefix(arg, "--") {
			name, value, hasValue := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
			flag := spec.flags[name]
			if flag == nil || name == "" {
				return Result{}, spec.unknownFlag(arg)
			}
			if err := spec.parseFlagValue(&result, flag, value, hasValue, true, args, &index); err != nil {
				return Result{}, err
			}
			continue
		}
		if parseFlags && strings.HasPrefix(arg, "-") {
			body := strings.TrimPrefix(arg, "-")
			shortName, value, hasValue := strings.Cut(body, "=")
			runes := []rune(shortName)
			if len(runes) != 1 {
				return Result{}, spec.unknownFlag(arg)
			}
			flag := spec.shorts[runes[0]]
			if flag == nil {
				return Result{}, spec.unknownFlag(arg)
			}
			if err := spec.parseFlagValue(&result, flag, value, hasValue, false, args, &index); err != nil {
				return Result{}, err
			}
			continue
		}
		result.positionals = append(result.positionals, arg)
	}

	if err := spec.validatePositionals(len(result.positionals)); err != nil {
		return Result{}, err
	}
	for _, flag := range spec.flagOrder {
		if flag.required && !result.seen[flag.name] {
			return Result{}, fmt.Errorf("%s requires --%s", spec.name(), flag.name)
		}
	}
	return result, nil
}

// ParseCommandLine parses the current process arguments after the executable name.
func (spec *Spec) ParseCommandLine() (Result, error) {
	args := os.Args
	if len(args) > 0 {
		args = args[1:]
	}
	return spec.Parse(args)
}

// String returns a parsed string flag value.
func (result Result) String(name string) string {
	return result.strings[name]
}

// Bool returns a parsed bool flag value.
func (result Result) Bool(name string) bool {
	return result.bools[name]
}

// IsSet reports whether a flag was explicitly present.
func (result Result) IsSet(name string) bool {
	return result.seen[name]
}

// LastFlag returns the last explicitly present flag among names.
func (result Result) LastFlag(names ...string) string {
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}
	for index := len(result.flagOrder) - 1; index >= 0; index-- {
		name := result.flagOrder[index]
		if wanted[name] {
			return name
		}
	}
	return ""
}

// Positionals returns a copy of parsed positional values.
func (result Result) Positionals() []string {
	return append([]string(nil), result.positionals...)
}

// Positional returns the positional value at index, or an empty string.
func (result Result) Positional(index int) string {
	if index < 0 || index >= len(result.positionals) {
		return ""
	}
	return result.positionals[index]
}

// Usage returns stable plain-text usage for the command.
func (spec *Spec) Usage() string {
	return spec.usage(0)
}

// UsageWidth returns stable plain-text usage with descriptions wrapped to width.
func (spec *Spec) UsageWidth(width int) string {
	if width <= 0 {
		return spec.Usage()
	}
	return spec.usage(width)
}

func (spec *Spec) usage(width int) string {
	name := spec.name()
	positionals := spec.positionalPlaceholders()

	var builder strings.Builder
	fmt.Fprintf(&builder, "Usage: %s [flags]", name)
	if positionals != "" {
		fmt.Fprintf(&builder, " %s", positionals)
	}
	builder.WriteString("\n\nFlags:\n")

	type row struct {
		display     string
		description string
	}
	rows := make([]row, 0, len(spec.flagOrder)+1)
	widest := 0
	for _, flag := range spec.flagOrder {
		display := flag.display()
		description := flag.usageDescription()
		rows = append(rows, row{display: display, description: description})
		if width := utf8.RuneCountInString(display); width > widest {
			widest = width
		}
	}
	helpDisplay := "-h, --help"
	rows = append(rows, row{display: helpDisplay, description: "show help"})
	if width := utf8.RuneCountInString(helpDisplay); width > widest {
		widest = width
	}

	const (
		minimumAlignedDescriptionWidth = 20
		stackedDescriptionIndent       = 4
	)
	descriptionColumn := 2 + widest + 2
	stackDescriptions := width > 0 && width-descriptionColumn < minimumAlignedDescriptionWidth

	for _, row := range rows {
		builder.WriteString("  " + row.display)
		if row.description == "" || (width > 0 && len(strings.Fields(row.description)) == 0) {
			builder.WriteByte('\n')
			continue
		}

		switch {
		case width <= 0:
			fmt.Fprintf(&builder, "%*s  %s", widest-utf8.RuneCountInString(row.display), "", row.description)
		case stackDescriptions:
			builder.WriteByte('\n')
			for _, line := range wrapText(row.description, width-stackedDescriptionIndent) {
				fmt.Fprintf(&builder, "%*s%s\n", stackedDescriptionIndent, "", line)
			}
			continue
		default:
			lines := wrapText(row.description, width-descriptionColumn)
			fmt.Fprintf(&builder, "%*s  %s", widest-utf8.RuneCountInString(row.display), "", lines[0])
			for _, line := range lines[1:] {
				fmt.Fprintf(&builder, "\n%*s%s", descriptionColumn, "", line)
			}
		}
		builder.WriteByte('\n')
	}
	return builder.String()
}

// Validate returns fluent definition errors.
func (spec *Spec) Validate() error {
	return errors.Join(spec.definitionErrs...)
}

func (spec *Spec) flag(name string, kind flagKind, options ...FlagOption) *Spec {
	flag := &flagSpec{name: strings.TrimSpace(name), kind: kind}
	if kind == flagKindString {
		flag.valueName = "<value>"
	}
	for _, option := range options {
		if option != nil {
			option(flag)
		}
	}

	switch {
	case flag.name == "":
		spec.definitionErrs = append(spec.definitionErrs, errors.New("flag name cannot be empty"))
	case flag.name == "help":
		spec.definitionErrs = append(spec.definitionErrs, errors.New(`reserved flag "--help"`))
	case spec.flags[flag.name] != nil:
		spec.definitionErrs = append(spec.definitionErrs, fmt.Errorf(`duplicate flag "--%s"`, flag.name))
	default:
		spec.flags[flag.name] = flag
	}

	if flag.short != 0 {
		switch {
		case flag.short == 'h':
			spec.definitionErrs = append(spec.definitionErrs, errors.New(`reserved short flag "-h"`))
		case spec.shorts[flag.short] != nil:
			spec.definitionErrs = append(spec.definitionErrs, fmt.Errorf(`duplicate short flag "-%c"`, flag.short))
		default:
			spec.shorts[flag.short] = flag
		}
	}
	if kind == flagKindBool && flag.hasDefault {
		if _, ok := maybeBoolValue(flag.defaultValue); !ok {
			spec.definitionErrs = append(spec.definitionErrs, fmt.Errorf("invalid default for --%s: %q", flag.name, flag.defaultValue))
		}
	}
	spec.flagOrder = append(spec.flagOrder, flag)
	return spec
}

func (spec *Spec) name() string {
	if spec.displayName != "" {
		return spec.displayName
	}
	return spec.command
}

func (spec *Spec) parseFlagValue(result *Result, flag *flagSpec, inlineValue string, hasInlineValue, allowSeparatedBool bool, args []string, index *int) error {
	switch flag.kind {
	case flagKindString:
		value := inlineValue
		if !hasInlineValue {
			*index++
			if *index >= len(args) {
				return fmt.Errorf("%s --%s requires a value", spec.name(), flag.name)
			}
			value = args[*index]
		}
		result.strings[flag.name] = value
	case flagKindBool:
		value := true
		if hasInlineValue {
			parsed, ok := maybeBoolValue(inlineValue)
			if !ok {
				return fmt.Errorf("invalid --%s value %q", flag.name, inlineValue)
			}
			value = parsed
		} else if allowSeparatedBool && *index+1 < len(args) {
			if parsed, ok := maybeBoolValue(args[*index+1]); ok {
				*index++
				value = parsed
			}
		}
		result.bools[flag.name] = value
	}
	result.seen[flag.name] = true
	result.flagOrder = append(result.flagOrder, flag.name)
	return nil
}

func (spec *Spec) unknownFlag(arg string) error {
	return fmt.Errorf("unknown %s flag %q", spec.name(), arg)
}

func (spec *Spec) validatePositionals(count int) error {
	if count < spec.minPositionals {
		return fmt.Errorf("%s requires %s", spec.name(), positionalList(spec.positionalNames, spec.minPositionals))
	}
	if spec.maxPositionals >= 0 && count > spec.maxPositionals {
		return fmt.Errorf("%s accepts %s", spec.name(), maxPositionalsText(spec.positionalNames, spec.maxPositionals))
	}
	return nil
}

func positionalList(names []string, count int) string {
	if count == 0 {
		return "no positionals"
	}
	formatted := make([]string, count)
	for index := range formatted {
		name := fmt.Sprintf("arg%d", index+1)
		if index < len(names) && names[index] != "" {
			name = names[index]
		}
		formatted[index] = "<" + name + ">"
	}
	if len(formatted) == 1 {
		return formatted[0]
	}
	return strings.Join(formatted[:len(formatted)-1], ", ") + " and " + formatted[len(formatted)-1]
}

func maxPositionalsText(names []string, count int) string {
	switch count {
	case 0:
		return "no positionals"
	case 1:
		name := "arg1"
		if len(names) > 0 && names[0] != "" {
			name = names[0]
		}
		return "one <" + name + ">"
	default:
		return fmt.Sprintf("%d positionals", count)
	}
}

func (spec *Spec) positionalPlaceholders() string {
	if spec.maxPositionals == 0 {
		return ""
	}
	count := spec.maxPositionals
	if count < 0 {
		count = len(spec.positionalNames)
		if spec.minPositionals > count {
			count = spec.minPositionals
		}
	}
	parts := make([]string, 0, count)
	for index := 0; index < count; index++ {
		name := fmt.Sprintf("arg%d", index+1)
		if index < len(spec.positionalNames) && spec.positionalNames[index] != "" {
			name = spec.positionalNames[index]
		}
		placeholder := "<" + name + ">"
		if index >= spec.minPositionals {
			placeholder = "[" + placeholder + "]"
		}
		parts = append(parts, placeholder)
	}
	return strings.Join(parts, " ")
}

func (flag *flagSpec) display() string {
	long := "--" + flag.name
	if flag.kind == flagKindString {
		long += " " + flag.valueName
	}
	if flag.short == 0 {
		return "    " + long
	}
	return fmt.Sprintf("-%c, %s", flag.short, long)
}

func (flag *flagSpec) usageDescription() string {
	description := flag.description
	if !flag.hasDefault {
		return description
	}
	defaultText := flag.defaultValue
	if flag.kind == flagKindString {
		defaultText = fmt.Sprintf("%q", defaultText)
	} else if parsed, ok := maybeBoolValue(defaultText); ok {
		defaultText = fmt.Sprint(parsed)
	}
	if description == "" {
		return "(default " + defaultText + ")"
	}
	return description + " (default " + defaultText + ")"
}

func valueLabel(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "<") && strings.HasSuffix(name, ">") {
		return name
	}
	return "<" + name + ">"
}

func wrapText(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	lines := make([]string, 0, len(words))
	line := words[0]
	for _, word := range words[1:] {
		candidate := line + " " + word
		if utf8.RuneCountInString(candidate) <= width {
			line = candidate
			continue
		}
		lines = append(lines, line)
		line = word
	}
	return append(lines, line)
}

func commandPath(parts ...string) string {
	var normalized []string
	for _, part := range parts {
		normalized = append(normalized, strings.Fields(strings.TrimSpace(part))...)
	}
	return strings.Join(normalized, " ")
}

func maybeBoolValue(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes":
		return true, true
	case "false", "0", "no":
		return false, true
	default:
		return false, false
	}
}
