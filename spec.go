package chomp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

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

// Int declares an int flag.
func (spec *Spec) Int(name string, options ...FlagOption) *Spec {
	return spec.flag(name, flagKindInt, options...)
}

// Duration declares a time.Duration flag.
func (spec *Spec) Duration(name string, options ...FlagOption) *Spec {
	return spec.flag(name, flagKindDuration, options...)
}

// Strings declares a repeated string flag.
func (spec *Spec) Strings(name string, options ...FlagOption) *Spec {
	return spec.flag(name, flagKindStrings, options...)
}

// Positionals declares the accepted positional count and usage names.
func (spec *Spec) Positionals(min, max int, names ...string) *Spec {
	spec.minPositionals = min
	spec.maxPositionals = max
	spec.positionalNames = append([]string(nil), names...)
	return spec
}

func (spec *Spec) flag(name string, kind flagKind, options ...FlagOption) *Spec {
	flag := &flagSpec{name: strings.TrimSpace(name), kind: kind}
	if flag.takesValue() {
		flag.valueName = flag.defaultValueName()
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
	if kind == flagKindInt && flag.hasDefault {
		if _, err := strconv.Atoi(flag.defaultValue); err != nil {
			spec.definitionErrs = append(spec.definitionErrs, fmt.Errorf("invalid default for --%s: %q", flag.name, flag.defaultValue))
		}
	}
	if kind == flagKindDuration && flag.hasDefault {
		if _, err := time.ParseDuration(flag.defaultValue); err != nil {
			spec.definitionErrs = append(spec.definitionErrs, fmt.Errorf("invalid default for --%s: %q", flag.name, flag.defaultValue))
		}
	}
	spec.validateOneOf(flag)
	spec.flagOrder = append(spec.flagOrder, flag)
	return spec
}

func (spec *Spec) name() string {
	if spec.displayName != "" {
		return spec.displayName
	}
	return spec.command
}

func (flag *flagSpec) takesValue() bool {
	return flag.kind == flagKindString || flag.kind == flagKindInt || flag.kind == flagKindDuration || flag.kind == flagKindStrings
}

func (flag *flagSpec) defaultValueName() string {
	switch flag.kind {
	case flagKindInt:
		return "<int>"
	case flagKindDuration:
		return "<duration>"
	default:
		return "<value>"
	}
}

func (flag flagKind) kindName() string {
	switch flag {
	case flagKindString:
		return "string"
	case flagKindBool:
		return "bool"
	case flagKindInt:
		return "int"
	case flagKindDuration:
		return "duration"
	case flagKindStrings:
		return "strings"
	default:
		return "unknown"
	}
}

func valueLabel(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "<") && strings.HasSuffix(name, ">") {
		return name
	}
	return "<" + name + ">"
}

func commandPath(parts ...string) string {
	var normalized []string
	for _, part := range parts {
		normalized = append(normalized, strings.Fields(strings.TrimSpace(part))...)
	}
	return strings.Join(normalized, " ")
}
