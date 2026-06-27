package chomp

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

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
			lines := strings.Split(row.description, "\n")
			fmt.Fprintf(&builder, "%*s  %s", widest-utf8.RuneCountInString(row.display), "", lines[0])
			for _, line := range lines[1:] {
				fmt.Fprintf(&builder, "\n%*s%s", descriptionColumn, "", line)
			}
		case stackDescriptions:
			builder.WriteByte('\n')
			for _, description := range strings.Split(row.description, "\n") {
				for _, line := range wrapText(description, width-stackedDescriptionIndent) {
					fmt.Fprintf(&builder, "%*s%s\n", stackedDescriptionIndent, "", line)
				}
			}
			continue
		default:
			lines := wrapDescription(row.description, width-descriptionColumn)
			fmt.Fprintf(&builder, "%*s  %s", widest-utf8.RuneCountInString(row.display), "", lines[0])
			for _, line := range lines[1:] {
				fmt.Fprintf(&builder, "\n%*s%s", descriptionColumn, "", line)
			}
		}
		builder.WriteByte('\n')
	}
	return builder.String()
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
	if flag.takesValue() {
		long += " " + flag.valueName
	}
	if flag.short == 0 {
		return "    " + long
	}
	return fmt.Sprintf("-%c, %s", flag.short, long)
}

func (flag *flagSpec) usageDescription() string {
	description := flag.description
	if flag.hasOneOf && len(flag.oneOf) > 0 {
		oneOf := "(" + flag.oneOfDescription() + ")"
		if description == "" {
			description = oneOf
		} else {
			description += " " + oneOf
		}
	}
	if !flag.hasDefault {
		return description
	}
	defaultText := flag.defaultValue
	if flag.kind == flagKindString || flag.kind == flagKindStrings {
		defaultText = fmt.Sprintf("%q", defaultText)
	} else if parsed, ok := maybeBoolValue(defaultText); ok {
		defaultText = fmt.Sprint(parsed)
	}
	if description == "" {
		return "(default " + defaultText + ")"
	}
	if flag.hasOneOf {
		return description + "\n(default " + defaultText + ")"
	}
	return description + " (default " + defaultText + ")"
}

func (flag *flagSpec) oneOfDescription() string {
	quoted := make([]string, len(flag.oneOf))
	for index, value := range flag.oneOf {
		quoted[index] = fmt.Sprintf("%q", value)
	}
	return "one of " + strings.Join(quoted, ", ")
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

func wrapDescription(description string, width int) []string {
	var lines []string
	for _, part := range strings.Split(description, "\n") {
		lines = append(lines, wrapText(part, width)...)
	}
	return lines
}
