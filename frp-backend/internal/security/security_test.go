package security

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_HashPassword_returns_non_empty_string(t *testing.T) {
	// When
	hash, err := HashPassword("my-secret-password")

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, hash)
}

func Test_HashPassword_contains_colon_separator(t *testing.T) {
	// When
	hash, err := HashPassword("test-password")

	// Then
	require.NoError(t, err)
	require.Contains(t, hash, ":")
	parts := strings.Split(hash, ":")
	require.Len(t, parts, 2)
	require.NotEmpty(t, parts[0]) // salt (hex)
	require.NotEmpty(t, parts[1]) // hash (hex)
}

func Test_HashPassword_produces_different_hashes_each_call(t *testing.T) {
	// Given
	password := "same-password"

	// When
	hash1, err1 := HashPassword(password)
	hash2, err2 := HashPassword(password)

	// Then
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NotEqual(t, hash1, hash2, "salts should be random, producing different hashes")
}

func Test_VerifyPassword_returns_true_for_correct_password(t *testing.T) {
	// Given
	password := "correct-password"
	hash, err := HashPassword(password)
	require.NoError(t, err)

	// When
	valid := VerifyPassword(password, hash)

	// Then
	require.True(t, valid)
}

func Test_VerifyPassword_returns_false_for_wrong_password(t *testing.T) {
	// Given
	hash, err := HashPassword("real-password")
	require.NoError(t, err)

	// When
	valid := VerifyPassword("wrong-password", hash)

	// Then
	require.False(t, valid)
}

func Test_VerifyPassword_returns_false_for_empty_password(t *testing.T) {
	// Given
	hash, err := HashPassword("some-password")
	require.NoError(t, err)

	// When
	valid := VerifyPassword("", hash)

	// Then
	require.False(t, valid)
}

func Test_VerifyPassword_returns_false_for_malformed_hash(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{"no colon separator", "abcdef1234"},
		{"too many parts", "a:b:c"},
		{"empty salt", ":abcdef"},
		{"empty hash", "abcdef:"},
		{"invalid hex in salt", "xyz:abc"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			valid := VerifyPassword("irrelevant", tt.hash)

			// Then
			require.False(t, valid, "expected false for hash %q", tt.hash)
		})
	}
}

func Test_HashToken_starts_with_tok_prefix(t *testing.T) {
	// When
	token := HashToken("my-session-token")

	// Then
	require.True(t, strings.HasPrefix(token, "tok_"), "expected 'tok_' prefix, got %q", token)
	require.Greater(t, len(token), 4)
}

func Test_HashToken_is_deterministic(t *testing.T) {
	// Given
	input := "deterministic-test"

	// When
	hash1 := HashToken(input)
	hash2 := HashToken(input)

	// Then
	require.Equal(t, hash1, hash2)
}

func Test_HashToken_different_inputs_different_hashes(t *testing.T) {
	// When
	hash1 := HashToken("token-a")
	hash2 := HashToken("token-b")

	// Then
	require.NotEqual(t, hash1, hash2)
}

func Test_RoundTrip_HashPassword_VerifyPassword(t *testing.T) {
	passwords := []string{
		"simple",
		"with spaces and 数字",
		"long-password-with-special-chars!@#$%^&*()",
		"a",
		"a-really-long-password-that-exceeds-typical-boundaries-because-we-need-to-test-that-too-1234567890",
	}

	for _, pwd := range passwords {
		t.Run(pwd[:min(len(pwd), 20)], func(t *testing.T) {
			// Given
			hash, err := HashPassword(pwd)
			require.NoError(t, err)

			// When
			valid := VerifyPassword(pwd, hash)

			// Then
			require.True(t, valid, "round-trip failed for password %q", pwd)
		})
	}
}
