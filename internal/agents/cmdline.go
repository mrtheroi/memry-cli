package agents

import (
	"errors"
	"strings"
)

// errUnsafeCmdArg is why a command line was refused.
var errUnsafeCmdArg = errors.New("an argument has a character cmd.exe would interpret: one of \" % ! CR LF NUL")

// CmdLine builds the command line that runs a .cmd or .bat script through
// cmd.exe with its arguments.
func CmdLine(script string, args []string) (string, error) {
	var line strings.Builder
	line.WriteString(`cmd.exe /d /s /c "`)
	for i, word := range append([]string{script}, args...) {
		if i > 0 {
			line.WriteByte(' ')
		}
		if strings.ContainsAny(word, "\"%!\r\n\x00") {
			return "", errUnsafeCmdArg
		}
		trimmed := strings.TrimRight(word, `\`)
		line.WriteString(`"` + word + word[len(trimmed):] + `"`)
	}
	line.WriteByte('"')
	return line.String(), nil
}
