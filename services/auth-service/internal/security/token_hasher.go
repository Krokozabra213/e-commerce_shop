package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type HashingService struct {
	secret []byte
}

func NewHashingService(secret string) *HashingService {
	return &HashingService{
		secret: []byte(secret),
	}
}

func (h *HashingService) sum(value string) []byte {
	mac := hmac.New(sha256.New, h.secret)
	mac.Write([]byte(value))
	return mac.Sum(nil)
}

func (h *HashingService) Hash(value string) string {
	return hex.EncodeToString(h.sum(value))
}

func (h *HashingService) Compare(inputToken string, storedHash string) (bool, error) {
	storedBytes, err := hex.DecodeString(storedHash)
	if err != nil {
		return false, fmt.Errorf("invalid stored hash: %w", err)
	}

	expectedBytes := h.sum(inputToken)

	if len(storedBytes) != len(expectedBytes) {
		return false, nil
	}

	return hmac.Equal(expectedBytes, storedBytes), nil
}
