package chomp

import "time"

// String returns a parsed string flag value.
func (result Result) String(name string) string {
	value, _ := result.values[name].(string)
	return value
}

// Bool returns a parsed bool flag value.
func (result Result) Bool(name string) bool {
	value, _ := result.values[name].(bool)
	return value
}

// Int returns a parsed int flag value.
func (result Result) Int(name string) int {
	value, _ := result.values[name].(int)
	return value
}

// Duration returns a parsed time.Duration flag value.
func (result Result) Duration(name string) time.Duration {
	value, _ := result.values[name].(time.Duration)
	return value
}

// Strings returns a copy of parsed repeated string flag values.
func (result Result) Strings(name string) []string {
	values, _ := result.values[name].([]string)
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
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
