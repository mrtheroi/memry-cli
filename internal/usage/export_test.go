package usage

// SetBeforeSettingsWrite makes enable call f right before it writes the
// settings, as a crash there would stop it, until the test ends.
func SetBeforeSettingsWrite(f func()) (restore func()) {
	before := beforeSettingsWrite
	beforeSettingsWrite = f
	return func() { beforeSettingsWrite = before }
}
