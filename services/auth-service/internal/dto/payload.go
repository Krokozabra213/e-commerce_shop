package dto

import (
	"encoding/json"
	"time"
)

type UserCreatedPayload struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	Source    string    `json:"source,omitempty"`
	Provider  string    `json:"provider,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func ToMap(v any) (map[string]any, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(bytes, &m); err != nil {
		return nil, err
	}
	return m, nil
}
