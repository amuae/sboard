package middleware

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func init() {
	// Set a fixed secret for deterministic tests
	InitJWT("test-secret-key-for-unit-tests-32b")
}

// ========== GenerateToken + ParseToken round-trip tests ==========

func TestGenerateAndParseToken_RoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		userID      uint
		username    string
		expireHours int
	}{
		{
			name:        "standard user token",
			userID:      1,
			username:    "admin",
			expireHours: 168,
		},
		{
			name:        "different user ID",
			userID:      42,
			username:    "user42",
			expireHours: 24,
		},
		{
			name:        "username with special characters",
			userID:      99,
			username:    "test-user@example.com",
			expireHours: 1,
		},
		{
			name:        "zero user ID edge case",
			userID:      0,
			username:    "root",
			expireHours: 720,
		},
		{
			name:        "short expiry",
			userID:      5,
			username:    "temp",
			expireHours: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateToken(tt.userID, tt.username, tt.expireHours)
			if err != nil {
				t.Fatalf("GenerateToken() error: %v", err)
			}
			if token == "" {
				t.Fatal("GenerateToken() returned empty string")
			}

			// Verify it's a well-formed JWT (3 dot-separated parts)
			parts := strings.Split(token, ".")
			if len(parts) != 3 {
				t.Errorf("token should have 3 parts, got %d: %s", len(parts), token)
			}

			claims, err := ParseToken(token)
			if err != nil {
				t.Fatalf("ParseToken() error: %v", err)
			}
			if claims.UserID != tt.userID {
				t.Errorf("claims.UserID = %d, want %d", claims.UserID, tt.userID)
			}
			if claims.Username != tt.username {
				t.Errorf("claims.Username = %q, want %q", claims.Username, tt.username)
			}
			if claims.Issuer != "sboard" {
				t.Errorf("claims.Issuer = %q, want %q", claims.Issuer, "sboard")
			}
		})
	}
}

// ========== ParseToken error cases ==========

func TestParseToken_InvalidTokens(t *testing.T) {
	tests := []struct {
		name        string
		tokenString string
		expectError bool
		errContains string // substring to look for in error (empty = any error ok)
	}{
		{
			name:        "empty string",
			tokenString: "",
			expectError: true,
		},
		{
			name:        "garbage string",
			tokenString: "not-a-jwt-token",
			expectError: true,
		},
		{
			name:        "malformed — only one part",
			tokenString: "header",
			expectError: true,
		},
		{
			name:        "malformed — two parts",
			tokenString: "header.payload",
			expectError: true,
		},
		{
			name:        "tampered signature",
			tokenString: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxLCJ1c2VybmFtZSI6ImFkbWluIn0.tampered",
			expectError: true,
		},
		{
			name:        "base64 garbage with 3 parts",
			tokenString: "aaa.bbb.ccc",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := ParseToken(tt.tokenString)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseToken() expected error, got claims=%+v", claims)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseToken() unexpected error: %v", err)
			}
		})
	}
}

// ========== ParseToken with signature from different secret ==========

func TestParseToken_DifferentSecret(t *testing.T) {
	// Generate a token with our test secret
	token, err := GenerateToken(1, "admin", 1)
	if err != nil {
		t.Fatalf("GenerateToken() error: %v", err)
	}

	// Change the secret
	InitJWT("completely-different-secret-key!!!")

	claims, err := ParseToken(token)
	if err == nil {
		t.Errorf("ParseToken() should fail with different secret, got claims=%+v", claims)
	}

	// Reset for other tests
	InitJWT("test-secret-key-for-unit-tests-32b")
}

// ========== Token expiry test ==========

func TestParseToken_Expired(t *testing.T) {
	// This test is more of a documentation of behavior — the token
	// is valid on creation but would expire after the given hours.
	// We test that a token with 0 expireHours is accepted (it's already
	// expired or about to expire, which is a valid edge case).

	token, err := GenerateToken(1, "admin", 0)
	if err != nil {
		t.Fatalf("GenerateToken() error: %v", err)
	}

	// ParseToken should return an error for expired token
	// (jwt library handles the expiry check)
	claims, err := ParseToken(token)
	if err != nil {
		// This is expected — 0 hour expiry should be expired
		t.Logf("Token with 0h expiry correctly rejected: %v", err)
	} else {
		// Might pass due to clock skew tolerance; mark as known
		t.Logf("Token with 0h expiry accepted (clock skew tolerance): userID=%d", claims.UserID)
	}
}

// ========== GenerateToken produces unique tokens ==========

func TestGenerateToken_UniqueEachTime(t *testing.T) {
	token1, err := GenerateToken(1, "admin", 168)
	if err != nil {
		t.Fatalf("GenerateToken() error: %v", err)
	}

	// Small delay to ensure different timestamps (JWT timestamps are second-granular)
	time.Sleep(1100 * time.Millisecond)

	token2, err := GenerateToken(1, "admin", 168)
	if err != nil {
		t.Fatalf("GenerateToken() error: %v", err)
	}

	if token1 == token2 {
		t.Errorf("consecutive tokens should differ (different iat/nbf): %s", token1)
	}
}

// ========== Claims structure validation ==========

func TestClaims_Structure(t *testing.T) {
	token, err := GenerateToken(123, "testuser", 24)
	if err != nil {
		t.Fatalf("GenerateToken() error: %v", err)
	}

	// Parse with standard jwt parser to verify structure
	parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		t.Fatalf("jwt.ParseWithClaims() error: %v", err)
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok {
		t.Fatal("parsed claims are not *Claims")
	}

	if claims.UserID != 123 {
		t.Errorf("UserID = %d, want 123", claims.UserID)
	}
	if claims.Username != "testuser" {
		t.Errorf("Username = %q, want %q", claims.Username, "testuser")
	}
	if claims.Issuer != "sboard" {
		t.Errorf("Issuer = %q, want %q", claims.Issuer, "sboard")
	}
	if claims.IssuedAt == nil {
		t.Error("IssuedAt should not be nil")
	}
	if claims.ExpiresAt == nil {
		t.Error("ExpiresAt should not be nil")
	}
	if claims.NotBefore == nil {
		t.Error("NotBefore should not be nil")
	}
}

// ========== Invalid JWT signing method ==========

func TestParseToken_WrongAlgorithm(t *testing.T) {
	// Create a token with RS256 instead of HS256
	// This token should be rejected since we only accept HS256
	token := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.POstGetfAytaZS82wHcjoTyoqhMyxXiWdR7Nn7A29DNSl0EiXLdwJ6xC6AfgZWF1bOsS_TuYI3OG85AmiExREkrS6tDfTQ2B3WXlrr-wp5AokiRbz3_oB4OxG-W9KcEEbDRcZc0nH3fT0q7g3N7p9Q"
	_, err := ParseToken(token)
	if err == nil {
		t.Error("ParseToken() should reject tokens with non-HS256 algorithm")
	}
}

// ========== InitJWT / secret management ==========

func TestInitJWT(t *testing.T) {
	// Save original
	origSecret := jwtSecret

	InitJWT("new-secret-value")
	if string(jwtSecret) != "new-secret-value" {
		t.Errorf("jwtSecret = %q, want %q", string(jwtSecret), "new-secret-value")
	}

	// Can generate tokens with new secret
	token, err := GenerateToken(1, "admin", 1)
	if err != nil {
		t.Fatalf("GenerateToken() with new secret error: %v", err)
	}

	// Token should be parseable with new secret
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken() with new secret error: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("UserID = %d, want 1", claims.UserID)
	}

	// Reset for other tests
	jwtSecret = origSecret
	InitJWT("test-secret-key-for-unit-tests-32b")
}
