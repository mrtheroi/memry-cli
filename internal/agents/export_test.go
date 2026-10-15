package agents

// SetGetwd makes ClaudeSettingsPath resolve a relative directory against
// what f returns, until the test ends.
func SetGetwd(f func() (string, error)) (restore func()) {
	before := getwd
	getwd = f
	return func() { getwd = before }
}
