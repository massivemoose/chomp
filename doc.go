// Package chomp provides small command-line argument parsing, command routing,
// and stable plain-text usage rendering.
//
// Chomp parses string, bool, int, duration, and repeated string flags alongside
// positional arguments. It supports exact string value validation and explicit
// nested command routing. It does not execute command business logic, bind
// configuration, or manage process lifecycle hooks.
package chomp
