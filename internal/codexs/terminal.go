package codexs

import (
	"strings"
)

type TerminalKind string

const (
	TerminalApple TerminalKind = "terminal"
	TerminalITerm TerminalKind = "iterm2"
)

func buildRunShellCommand(executable, profileName string, codexArgs []string) string {
	parts := []string{shellQuote(executable), "run", shellQuote(profileName)}
	if len(codexArgs) > 0 {
		parts = append(parts, "--")
		for _, arg := range codexArgs {
			parts = append(parts, shellQuote(arg))
		}
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
