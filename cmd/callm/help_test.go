package main

import (
	"os"
	"strings"
	"testing"
)

// Keep the published CLI reference identical to the help users actually receive.
func TestREADMEHelpReference(t *testing.T) {
	output, err := testCLI(t, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("help: %v: %s", err, output)
	}
	help := string(output)
	start, end := strings.Index(help, "Usage:\n"), strings.Index(help, "\nExamples:\n")
	if start < 0 || end <= start {
		t.Fatal("help reference boundaries missing")
	}
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	expected := "<!-- CLI-HELP:START -->\n```text\n" + help[start:end] + "\n```\n<!-- CLI-HELP:END -->"
	if !strings.Contains(string(readme), expected) {
		t.Fatal("README CLI reference differs from --help; update the CLI-HELP block")
	}
}

func TestSubcommandHelpTimeouts(t *testing.T) {
	for _, command := range []string{"models", "info", "raw"} {
		t.Run(command, func(t *testing.T) {
			output, err := testCLI(t, command, "--help").CombinedOutput()
			if err != nil {
				t.Fatalf("help: %v: %s", err, output)
			}
			for _, want := range []string{"Usage: callm " + command + " [OPTIONS]", "default 300s", "inherits --timeout"} {
				if !strings.Contains(string(output), want) {
					t.Fatalf("help lacks %q: %s", want, output)
				}
			}
		})
	}
}
