package kafka

import (
	"fmt"

	"github.com/google/uuid"
)

func parseUUID(s string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, fmt.Errorf("empty uuid string")
	}

	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid uuid: %w", err)
	}

	return id, nil
}
