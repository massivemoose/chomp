package chomp

import (
	"errors"
	"fmt"
)

// Validate returns fluent definition errors.
func (spec *Spec) Validate() error {
	return errors.Join(spec.definitionErrs...)
}

func (spec *Spec) validateOneOf(flag *flagSpec) {
	if !flag.hasOneOf {
		return
	}
	if flag.kind != flagKindString && flag.kind != flagKindStrings {
		spec.definitionErrs = append(spec.definitionErrs, fmt.Errorf("--%s cannot use OneOf on %s flag", flag.name, flag.kind.kindName()))
		return
	}
	if len(flag.oneOf) == 0 {
		spec.definitionErrs = append(spec.definitionErrs, fmt.Errorf("--%s OneOf requires at least one value", flag.name))
		return
	}
	seen := make(map[string]bool, len(flag.oneOf))
	for _, value := range flag.oneOf {
		if seen[value] {
			spec.definitionErrs = append(spec.definitionErrs, fmt.Errorf("duplicate OneOf value %q for --%s", value, flag.name))
			return
		}
		seen[value] = true
	}
	if flag.hasDefault && !flag.allows(flag.defaultValue) {
		spec.definitionErrs = append(spec.definitionErrs, fmt.Errorf("invalid default for --%s: %q; expected %s", flag.name, flag.defaultValue, flag.oneOfDescription()))
	}
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

func (flag *flagSpec) validateStringValue(value string) error {
	if !flag.hasOneOf || flag.allows(value) {
		return nil
	}
	return fmt.Errorf("invalid --%s value %q: expected %s", flag.name, value, flag.oneOfDescription())
}

func (flag *flagSpec) allows(value string) bool {
	for _, allowed := range flag.oneOf {
		if allowed == value {
			return true
		}
	}
	return false
}
