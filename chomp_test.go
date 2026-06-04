package chomp

import (
	"errors"
	"strings"
	"testing"
)

func TestUsageNormalizesCommandMetadataAndUsesDisplayName(t *testing.T) {
	spec := New(" chomp  ", "  render\nfile ").
		String("format", Short('f'), ValueName("format"), Description("output format"), Default("json")).
		Bool("verbose", Short('v'), Description("show details"), Default("true")).
		Positionals(1, 2, "input", "output")

	const want = `Usage: chomp render file [flags] <input> [<output>]

Flags:
  -f, --format <format>  output format (default "json")
  -v, --verbose          show details (default true)
  -h, --help             show help
`
	if got := spec.Usage(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}

	spec.DisplayName("tool render")
	if got := spec.Usage(); !strings.HasPrefix(got, "Usage: tool render [flags]") {
		t.Fatalf("expected display name in usage, got:\n%s", got)
	}
}

func TestUsageUsesDefaultStringValueNameAndDeclarationOrder(t *testing.T) {
	spec := New("tool").
		Bool("z-last").
		String("a-first", Required(), Description("required value")).
		String("bracketed", ValueName("<item>"))

	const want = `Usage: tool [flags]

Flags:
      --z-last
      --a-first <value>   required value
      --bracketed <item>
  -h, --help              show help
`
	if got := spec.Usage(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestUsageUnboundedPositionalsCoverRequiredMinimum(t *testing.T) {
	spec := New("tool").Positionals(2, -1, "input")

	const want = `Usage: tool [flags] <input> <arg2>

Flags:
  -h, --help  show help
`
	if got := spec.Usage(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestUsageAlignsUnicodeFlagDisplaysByRuneCount(t *testing.T) {
	spec := New("tool").
		Bool("é", Short('界'), Description("unicode")).
		Bool("long", Description("ascii"))

	const want = `Usage: tool [flags]

Flags:
  -界, --é     unicode
      --long  ascii
  -h, --help  show help
`
	if got := spec.Usage(); got != want {
		t.Fatalf("unexpected usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestValidateAcceptsValidDefinition(t *testing.T) {
	err := New("tool").
		String("output", Short('o'), Default(""), ValueName("path")).
		Bool("verbose", Short('v'), Default("yes")).
		Validate()
	if err != nil {
		t.Fatalf("expected valid definition, got %v", err)
	}
}

func TestValidateRecordsDefinitionErrors(t *testing.T) {
	tests := []struct {
		name string
		spec *Spec
		want string
	}{
		{"empty long", New("tool").String(""), "flag name cannot be empty"},
		{"duplicate long", New("tool").String("output").Bool("output"), `duplicate flag "--output"`},
		{"duplicate short", New("tool").String("output", Short('o')).Bool("overwrite", Short('o')), `duplicate short flag "-o"`},
		{"reserved long help", New("tool").Bool("help"), `reserved flag "--help"`},
		{"reserved short help", New("tool").Bool("hello", Short('h')), `reserved short flag "-h"`},
		{"invalid bool default", New("tool").Bool("verbose", Default("sometimes")), `invalid default for --verbose: "sometimes"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}

			_, parseErr := tt.spec.Parse(nil)
			if parseErr == nil || parseErr.Error() != err.Error() {
				t.Fatalf("expected Parse to return validation error %q, got %v", err, parseErr)
			}
		})
	}
}

func TestParseReturnsHelpError(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}} {
		_, err := New("tool").Parse(args)
		if !errors.Is(err, ErrHelp) {
			t.Fatalf("expected ErrHelp for %#v, got %v", args, err)
		}
	}
}

func TestParseAppliesDefaultsWithoutMarkingFlagsSet(t *testing.T) {
	result, err := New("tool").
		String("format", Default("json")).
		String("empty", Default("")).
		Bool("enabled", Default("yes")).
		Bool("disabled", Default("0")).
		Parse(nil)
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}

	if got := result.String("format"); got != "json" {
		t.Fatalf("expected string default, got %q", got)
	}
	if got := result.String("empty"); got != "" {
		t.Fatalf("expected empty string default, got %q", got)
	}
	if !result.Bool("enabled") || result.Bool("disabled") {
		t.Fatalf("unexpected bool defaults: enabled=%t disabled=%t", result.Bool("enabled"), result.Bool("disabled"))
	}
	for _, name := range []string{"format", "empty", "enabled", "disabled"} {
		if result.IsSet(name) {
			t.Fatalf("expected defaulted flag %q not to be set", name)
		}
	}
}

func TestParseRequiredMeansPresentEvenForEmptyOrFalseValues(t *testing.T) {
	result, err := New("tool").
		String("output", Required()).
		Bool("enabled", Required()).
		Parse([]string{"--output=", "--enabled=false"})
	if err != nil {
		t.Fatalf("expected explicit empty and false values to satisfy required, got %v", err)
	}
	if got := result.String("output"); got != "" {
		t.Fatalf("expected explicit empty string, got %q", got)
	}
	if result.Bool("enabled") {
		t.Fatal("expected explicit false bool")
	}
	if !result.IsSet("output") || !result.IsSet("enabled") {
		t.Fatal("expected explicit values to be marked set")
	}

	_, err = New("tool").String("output", Required()).Parse(nil)
	if err == nil || err.Error() != "tool requires --output" {
		t.Fatalf("expected missing required flag error, got %v", err)
	}
}

func TestParsePreservesRawStringAndPositionalValues(t *testing.T) {
	result, err := New("tool").
		String("inline").
		String("separated").
		Positionals(2, 2, "first", "second").
		Parse([]string{"--inline=  raw value  ", "--separated", "\tspaced\t", "", "  positional  "})
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}

	if got := result.String("inline"); got != "  raw value  " {
		t.Fatalf("expected raw inline value, got %q", got)
	}
	if got := result.String("separated"); got != "\tspaced\t" {
		t.Fatalf("expected raw separated value, got %q", got)
	}
	if got := result.Positionals(); len(got) != 2 || got[0] != "" || got[1] != "  positional  " {
		t.Fatalf("expected raw positionals, got %#v", got)
	}
}

func TestParseRejectsMissingSeparatedStringValue(t *testing.T) {
	_, err := New("tool").String("output").Parse([]string{"--output"})
	if err == nil || err.Error() != "tool --output requires a value" {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func TestParseAcceptsShortFlagFormsAndUsesCanonicalNames(t *testing.T) {
	result, err := New("tool").
		String("output", Short('o')).
		String("format", Short('f')).
		Bool("verbose", Short('v')).
		Bool("color", Short('c')).
		Parse([]string{"-o", "first", "-f=json", "-v=false", "-c"})
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}

	if got := result.String("output"); got != "first" {
		t.Fatalf("expected short separated string, got %q", got)
	}
	if got := result.String("format"); got != "json" {
		t.Fatalf("expected short inline string, got %q", got)
	}
	if result.Bool("verbose") || !result.Bool("color") {
		t.Fatalf("unexpected bool values: verbose=%t color=%t", result.Bool("verbose"), result.Bool("color"))
	}
	if got := result.LastFlag("format", "verbose", "color"); got != "color" {
		t.Fatalf("expected canonical last flag name, got %q", got)
	}
}

func TestParseShortBoolDoesNotConsumeSeparatedBoolValue(t *testing.T) {
	result, err := New("tool").
		Bool("verbose", Short('v')).
		Positionals(1, 1, "value").
		Parse([]string{"-v", "false"})
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}
	if !result.Bool("verbose") {
		t.Fatal("expected short bool flag to be true")
	}
	if got := result.Positional(0); got != "false" {
		t.Fatalf("expected separated bool text to remain positional, got %q", got)
	}
}

func TestParseLongBoolConsumesSeparatedBoolValue(t *testing.T) {
	result, err := New("tool").
		Bool("verbose").
		Positionals(0, 0).
		Parse([]string{"--verbose", "false"})
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}
	if result.Bool("verbose") {
		t.Fatal("expected separated long bool value to be false")
	}
	if got := result.Positionals(); len(got) != 0 {
		t.Fatalf("expected separated long bool value excluded from positionals, got %#v", got)
	}
}

func TestParseRejectsShortClusters(t *testing.T) {
	_, err := New("tool").Bool("verbose", Short('v')).Bool("all", Short('a')).Parse([]string{"-va"})
	if err == nil || err.Error() != `unknown tool flag "-va"` {
		t.Fatalf("expected cluster rejection, got %v", err)
	}
}

func TestParseStopsFlagParsingAfterDoubleDash(t *testing.T) {
	result, err := New("tool").
		Bool("verbose", Short('v')).
		Positionals(3, 3, "first", "second", "third").
		Parse([]string{"-v", "--", "--help", "-v", ""})
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}
	if !result.Bool("verbose") {
		t.Fatal("expected pre-double-dash flag")
	}
	if got := result.Positionals(); len(got) != 3 || got[0] != "--help" || got[1] != "-v" || got[2] != "" {
		t.Fatalf("unexpected positionals after double dash: %#v", got)
	}
}

func TestParseAcceptsBoolSpellings(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"1", true},
		{"yes", true},
		{"TRUE", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{" NO ", false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result, err := New("tool").Bool("enabled").Parse([]string{"--enabled=" + tt.value})
			if err != nil {
				t.Fatalf("expected parse to succeed, got %v", err)
			}
			if got := result.Bool("enabled"); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestParseRejectsInvalidBoolValue(t *testing.T) {
	_, err := New("tool").Bool("enabled").Parse([]string{"--enabled=maybe"})
	if err == nil || err.Error() != `invalid --enabled value "maybe"` {
		t.Fatalf("expected invalid bool error, got %v", err)
	}
}

func TestParseUsesLastRepeatedValue(t *testing.T) {
	result, err := New("tool").
		String("format", Short('f')).
		Bool("verbose", Short('v')).
		Parse([]string{"--format=old", "-f", "new", "--verbose=false", "-v"})
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}
	if got := result.String("format"); got != "new" {
		t.Fatalf("expected last string value, got %q", got)
	}
	if !result.Bool("verbose") {
		t.Fatal("expected last bool value")
	}
	if got := result.LastFlag("format", "verbose"); got != "verbose" {
		t.Fatalf("expected last repeated flag, got %q", got)
	}
}

func TestParseUsesGenericFullOrDisplayNameInErrors(t *testing.T) {
	_, err := New("tool", "render").Parse([]string{"--wat"})
	if err == nil || err.Error() != `unknown tool render flag "--wat"` {
		t.Fatalf("expected full command error, got %v", err)
	}

	_, err = New("tool", "render").DisplayName("render").Parse([]string{"--wat"})
	if err == nil || err.Error() != `unknown render flag "--wat"` {
		t.Fatalf("expected display-name error, got %v", err)
	}
}

func TestParseRejectsTooFewAndTooManyPositionals(t *testing.T) {
	_, err := New("tool", "render").Positionals(2, 2, "input", "output").Parse([]string{"input"})
	if err == nil || err.Error() != "tool render requires <input> and <output>" {
		t.Fatalf("expected too-few error, got %v", err)
	}

	_, err = New("tool", "render").Positionals(1, 1, "input").Parse([]string{"input", "extra"})
	if err == nil || err.Error() != "tool render accepts one <input>" {
		t.Fatalf("expected too-many error, got %v", err)
	}
}

func TestResultPositionalAccessorsReturnValuesAndCopies(t *testing.T) {
	result, err := New("tool").Positionals(2, 2, "first", "second").Parse([]string{"one", "two"})
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}
	if got := result.Positional(0); got != "one" {
		t.Fatalf("expected first positional, got %q", got)
	}
	if got := result.Positional(-1); got != "" {
		t.Fatalf("expected empty out-of-range positional, got %q", got)
	}
	positionals := result.Positionals()
	positionals[0] = "changed"
	if got := result.Positional(0); got != "one" {
		t.Fatalf("expected defensive positional copy, got %q", got)
	}
}
