package codexs

import (
	"strings"
)

func buildRunShellCommand(executable, profileName string, codexArgs []string) string {
	var b strings.Builder
	b.Grow(len(executable) + len(profileName) + 20) // Preallocate
	b.WriteString(shellQuote(executable))
	b.WriteString(" run ")
	b.WriteString(shellQuote(profileName))
	if len(codexArgs) > 0 {
		b.WriteString(" --")
		for _, arg := range codexArgs {
			b.WriteByte(' ')
			b.WriteString(shellQuote(arg))
		}
	}
	return b.String()
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	var b strings.Builder
	b.Grow(len(value) + 10) // Preallocate with buffer for quotes and escapes
	b.WriteByte('\'')
	b.WriteString(strings.ReplaceAll(value, "'", "'\\''"))
	b.WriteByte('\'')
	return b.String()
}
