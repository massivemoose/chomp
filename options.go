package chomp

// FlagOption configures a flag.
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

// OneOf restricts a string or repeated string flag to the provided values.
func OneOf(values ...string) FlagOption {
	return func(flag *flagSpec) {
		flag.oneOf = append([]string(nil), values...)
		flag.hasOneOf = true
	}
}
