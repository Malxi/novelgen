package store

import (
	"errors"
	"strings"
)

// ValidateKeyPart validates an identifier that becomes one path element in the
// project layout (a chapter ID, draft ID, recap ID, and so on). Project keys are
// not arbitrary paths: allowing a separator or ".." would let external input
// escape the project root on OSFS and would be meaningless to a key-value
// backend.
func ValidateKeyPart(value string) error {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return errors.New("key is required")
	case value == ".", value == "..":
		return errors.New("key must not be a relative path element")
	case strings.ContainsAny(value, `/\`):
		return errors.New("key must not contain path separators")
	case strings.ContainsRune(value, '\x00'):
		return errors.New("key must not contain NUL")
	}
	return nil
}
