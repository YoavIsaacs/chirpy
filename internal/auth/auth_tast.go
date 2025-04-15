package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestMakeJWT(t *testing.T) {
	// Setup
	userID := uuid.New()
	tokenSecret := "test-secret-key"
	expiresIn := time.Hour * 24

	// Test normal operation
	t.Run("Creates valid JWT", func(t *testing.T) {
		// Execute the function
		token, err := MakeJWT(userID, tokenSecret, expiresIn)
		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if token == "" {
			t.Fatal("Expected non-empty token")
		}

		// Parse the token to verify its contents
		claims := &jwt.RegisteredClaims{}
		parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(tokenSecret), nil
		})
		if err != nil {
			t.Fatalf("Error parsing token: %v", err)
		}
		if !parsedToken.Valid {
			t.Fatal("Expected valid token")
		}
		if claims.Issuer != "chirpy" {
			t.Fatalf("Expected issuer 'chirpy', got '%s'", claims.Issuer)
		}
		if claims.Subject != userID.String() {
			t.Fatalf("Expected subject '%s', got '%s'", userID.String(), claims.Subject)
		}

		// Verify the expiry time
		expectedExpiry := time.Now().UTC().Add(expiresIn)
		timeDiff := expectedExpiry.Sub(claims.ExpiresAt.Time)
		if timeDiff < -2*time.Second || timeDiff > 2*time.Second {
			t.Fatalf("Expiry time outside the expected range. Expected around %v, got %v", expectedExpiry, claims.ExpiresAt.Time)
		}
	})

	// Test with empty secret
	t.Run("Empty token secret", func(t *testing.T) {
		token, err := MakeJWT(userID, "", expiresIn)

		if err == nil {
			t.Fatal("Expected error with empty secret, got nil")
		}
		if token != "" {
			t.Fatalf("Expected empty token, got '%s'", token)
		}
	})
}

func TestValidateJWT(t *testing.T) {
	// Setup
	userID := uuid.New()
	tokenSecret := "test-secret-key"
	expiresIn := time.Hour * 24

	// Create a valid token for testing
	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("Failed to create token for testing: %v", err)
	}
	if token == "" {
		t.Fatal("Failed to create token for testing: empty token")
	}

	// Test with valid token
	t.Run("Valid token returns correct UUID", func(t *testing.T) {
		returnedID, err := ValidateJWT(token, tokenSecret)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if returnedID != userID {
			t.Fatalf("Expected user ID %v, got %v", userID, returnedID)
		}
	})

	// Test with invalid token
	t.Run("Invalid token string", func(t *testing.T) {
		returnedID, err := ValidateJWT("invalid-token", tokenSecret)

		if err == nil {
			t.Fatal("Expected error with invalid token, got nil")
		}
		if returnedID != uuid.Nil {
			t.Fatalf("Expected nil UUID, got %v", returnedID)
		}
	})

	// Test with wrong secret
	t.Run("Wrong token secret", func(t *testing.T) {
		returnedID, err := ValidateJWT(token, "wrong-secret")

		if err == nil {
			t.Fatal("Expected error with wrong secret, got nil")
		}
		if returnedID != uuid.Nil {
			t.Fatalf("Expected nil UUID, got %v", returnedID)
		}
	})

	// Test with expired token
	t.Run("Expired token", func(t *testing.T) {
		// Create a token that expires immediately
		expiredToken, err := MakeJWT(userID, tokenSecret, -1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to create expired token for testing: %v", err)
		}

		returnedID, err := ValidateJWT(expiredToken, tokenSecret)

		if err == nil {
			t.Fatal("Expected error with expired token, got nil")
		}
		if returnedID != uuid.Nil {
			t.Fatalf("Expected nil UUID, got %v", returnedID)
		}
	})

	// Test with tampered token
	t.Run("Tampered token", func(t *testing.T) {
		// Create a valid token and tamper with it
		parts := []byte(token)
		// Change a character in the middle of the token
		if len(parts) > 10 {
			parts[10] = byte('X')
		}
		tamperedToken := string(parts)

		returnedID, err := ValidateJWT(tamperedToken, tokenSecret)

		if err == nil {
			t.Fatal("Expected error with tampered token, got nil")
		}
		if returnedID != uuid.Nil {
			t.Fatalf("Expected nil UUID, got %v", returnedID)
		}
	})
}
