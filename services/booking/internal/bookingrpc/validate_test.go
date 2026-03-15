package bookingrpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidPhone(t *testing.T) {
	valid := []string{
		"+79991234567",
		"79991234567",
		"1234567890",
		"+12345678901234",
		"123456789012345",
	}
	for _, p := range valid {
		assert.True(t, isValidPhone(p), "expected valid: %s", p)
	}

	invalid := []string{
		"",
		"123456789",    // 9 digits — too short
		"1234567890123456", // 16 digits — too long
		"+7 999 123",   // spaces
		"+7-999-123",   // dashes
		"abc123",       // letters
		"+",            // only plus
	}
	for _, p := range invalid {
		assert.False(t, isValidPhone(p), "expected invalid: %s", p)
	}
}
