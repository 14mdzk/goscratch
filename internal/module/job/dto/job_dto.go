package dto

import "encoding/json"

// DispatchJobRequest represents the request to dispatch a new job
type DispatchJobRequest struct {
	Type     string          `json:"type" validate:"required"`
	Payload  json.RawMessage `json:"payload" validate:"required"`
	MaxRetry *int            `json:"max_retry,omitempty"`
}

// JobResponse represents the response after dispatching a job
type JobResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// JobTypeInfo describes an available job type
type JobTypeInfo struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ListJobTypesResponse represents the response for listing available job types
type ListJobTypesResponse struct {
	Types []JobTypeInfo `json:"types"`
}
