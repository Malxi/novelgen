package cmd

// containsString reports whether values contains want. It is a test helper
// shared by the cmd package tests.
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
