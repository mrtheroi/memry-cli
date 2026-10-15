package agentfiles

// SetBeforeReplace makes the settings writes call f once they have read
// the file again and are about to replace it, until the test ends.
func SetBeforeReplace(f func()) (restore func()) {
	before := beforeReplace
	beforeReplace = f
	return func() { beforeReplace = before }
}
