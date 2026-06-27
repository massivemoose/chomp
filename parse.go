package chomp

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Parse parses command arguments according to the specification.
func (spec *Spec) Parse(args []string) (Result, error) {
	if err := spec.Validate(); err != nil {
		return Result{}, err
	}

	result := Result{
		values: make(map[string]any),
		seen:   make(map[string]bool),
	}
	for _, flag := range spec.flagOrder {
		if !flag.hasDefault {
			continue
		}
		switch flag.kind {
		case flagKindString:
			result.values[flag.name] = flag.defaultValue
		case flagKindBool:
			result.values[flag.name], _ = maybeBoolValue(flag.defaultValue)
		case flagKindInt:
			result.values[flag.name], _ = strconv.Atoi(flag.defaultValue)
		case flagKindDuration:
			result.values[flag.name], _ = time.ParseDuration(flag.defaultValue)
		case flagKindStrings:
			result.values[flag.name] = []string{flag.defaultValue}
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
		if err := flag.validateStringValue(value); err != nil {
			return err
		}
		result.values[flag.name] = value
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
		result.values[flag.name] = value
	case flagKindInt:
		value := inlineValue
		if !hasInlineValue {
			*index++
			if *index >= len(args) {
				return fmt.Errorf("%s --%s requires a value", spec.name(), flag.name)
			}
			value = args[*index]
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid --%s value %q: expected int", flag.name, value)
		}
		result.values[flag.name] = parsed
	case flagKindDuration:
		value := inlineValue
		if !hasInlineValue {
			*index++
			if *index >= len(args) {
				return fmt.Errorf("%s --%s requires a value", spec.name(), flag.name)
			}
			value = args[*index]
		}
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid --%s value %q: expected duration", flag.name, value)
		}
		result.values[flag.name] = parsed
	case flagKindStrings:
		value := inlineValue
		if !hasInlineValue {
			*index++
			if *index >= len(args) {
				return fmt.Errorf("%s --%s requires a value", spec.name(), flag.name)
			}
			value = args[*index]
		}
		if err := flag.validateStringValue(value); err != nil {
			return err
		}
		values, _ := result.values[flag.name].([]string)
		if !result.seen[flag.name] {
			values = nil
		}
		result.values[flag.name] = append(values, value)
	}
	result.seen[flag.name] = true
	result.flagOrder = append(result.flagOrder, flag.name)
	return nil
}

func (spec *Spec) unknownFlag(arg string) error {
	return fmt.Errorf("unknown %s flag %q", spec.name(), arg)
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
