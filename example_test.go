package chomp_test

import (
	"errors"
	"fmt"

	"github.com/massivemoose/chomp"
)

func Example() {
	spec := chomp.New("report").
		String("format",
			chomp.Short('f'),
			chomp.Default("table"),
			chomp.Description("output format"),
		).
		Bool("verbose", chomp.Short('v'), chomp.Description("show extra detail")).
		Positionals(1, 1, "input")

	result, _ := spec.Parse([]string{"-v", "input.csv"})
	fmt.Println(result.String("format"), result.Bool("verbose"), result.Positional(0))
	// Output: table true input.csv
}

func ExampleErrHelp() {
	spec := chomp.New("report").Positionals(1, 1, "input")
	_, err := spec.Parse([]string{"--help"})

	fmt.Println(errors.Is(err, chomp.ErrHelp))
	fmt.Print(spec.Usage())
	// Output:
	// true
	// Usage: report [flags] <input>
	//
	// Flags:
	//   -h, --help  show help
}
