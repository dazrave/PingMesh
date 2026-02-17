package cluster

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/pingmesh/pingmesh/internal/store"
)

// GenerateJoinToken creates a one-time-use token for a new node to join.
func GenerateJoinToken(st store.Store, coordinatorAddr string, expiry time.Duration) (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generating secret: %w", err)
	}

	token := &model.JoinToken{
		CoordinatorAddr: coordinatorAddr,
		Secret:          secret,
		ExpiresAt:       time.Now().Add(expiry),
	}

	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("marshalling token: %w", err)
	}

	// Store hash of secret for validation
	hash := sha256.Sum256(secret)
	hashStr := base64.StdEncoding.EncodeToString(hash[:])

	if err := st.StoreJoinToken(hashStr, token.ExpiresAt.UnixMilli()); err != nil {
		return "", fmt.Errorf("storing token: %w", err)
	}

	return base64.StdEncoding.EncodeToString(tokenJSON), nil
}

// DecodeJoinToken decodes a base64-encoded join token.
func DecodeJoinToken(tokenStr string) (*model.JoinToken, error) {
	tokenJSON, err := base64.StdEncoding.DecodeString(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("decoding token: %w", err)
	}

	var token model.JoinToken
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	return &token, nil
}

// ValidateJoinToken checks if a join token is valid and consumes it.
func ValidateJoinToken(st store.Store, secret []byte) (bool, error) {
	hash := sha256.Sum256(secret)
	hashStr := base64.StdEncoding.EncodeToString(hash[:])
	return st.ValidateAndConsumeToken(hashStr)
}
