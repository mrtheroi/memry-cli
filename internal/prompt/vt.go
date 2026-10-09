package prompt

// Windows console output modes (consoleapi.h), kept here so vtMode is
// tested on every OS.
const (
	enableProcessedOutput           = 0x1
	enableVirtualTerminalProcessing = 0x4
)

// vtMode is the console mode that makes Windows interpret escape
// sequences: virtual terminal processing, which needs processed output.
func vtMode(mode uint32) uint32 {
	return mode | enableProcessedOutput | enableVirtualTerminalProcessing
}
