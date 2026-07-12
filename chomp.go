package chomp

import "errors"

// ErrHelp is returned when help was requested.
var ErrHelp = errors.New("help")

// Spec defines a command's accepted flags and positionals. After configuration
// is complete, a Spec may be parsed, validated, and rendered concurrently.
// Configuration methods must not run concurrently with those operations.
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
	oneOf        []string
	hasOneOf     bool
}

type flagKind int

const (
	flagKindString flagKind = iota
	flagKindBool
	flagKindInt
	flagKindDuration
	flagKindStrings
)

// Result contains parsed flag and positional values.
type Result struct {
	values      map[string]any
	seen        map[string]bool
	flagOrder   []string
	positionals []string
}
