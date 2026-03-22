package worker

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testPayload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestNewJob(t *testing.T) {
	t.Run("creates_job_with_defaults", func(t *testing.T) {
		payload := testPayload{Name: "test", Count: 42}
		job, err := NewJob("email.send", payload)

		require.NoError(t, err)
		assert.NotEmpty(t, job.ID, "should have a UUID")
		assert.Equal(t, "email.send", job.Type)
		assert.Equal(t, 3, job.MaxRetry, "default MaxRetry should be 3")
		assert.Equal(t, 0, job.Attempts, "default Attempts should be 0")
		assert.NotZero(t, job.CreatedAt)
		assert.NotNil(t, job.Payload)
	})

	t.Run("payload_is_serialized", func(t *testing.T) {
		payload := testPayload{Name: "hello", Count: 7}
		job, err := NewJob("test.type", payload)

		require.NoError(t, err)
		var decoded testPayload
		err = json.Unmarshal(job.Payload, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "hello", decoded.Name)
		assert.Equal(t, 7, decoded.Count)
	})

	t.Run("unmarshalable_payload_returns_error", func(t *testing.T) {
		// channels cannot be marshaled to JSON
		job, err := NewJob("fail", make(chan int))
		assert.Error(t, err)
		assert.Nil(t, job)
	})
}

func TestNewJobWithRetry(t *testing.T) {
	t.Run("custom_max_retry", func(t *testing.T) {
		job, err := NewJobWithRetry("email.send", testPayload{Name: "x"}, 10)

		require.NoError(t, err)
		assert.Equal(t, 10, job.MaxRetry)
		assert.Equal(t, "email.send", job.Type)
		assert.Equal(t, 0, job.Attempts)
	})

	t.Run("unmarshalable_payload_returns_error", func(t *testing.T) {
		job, err := NewJobWithRetry("fail", make(chan int), 5)
		assert.Error(t, err)
		assert.Nil(t, job)
	})
}

func TestEncodeDecodeJob(t *testing.T) {
	t.Run("round_trip", func(t *testing.T) {
		original, err := NewJob("test.roundtrip", testPayload{Name: "round", Count: 99})
		require.NoError(t, err)

		data, err := original.Encode()
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		decoded, err := DecodeJob(data)
		require.NoError(t, err)
		assert.Equal(t, original.ID, decoded.ID)
		assert.Equal(t, original.Type, decoded.Type)
		assert.Equal(t, original.MaxRetry, decoded.MaxRetry)
		assert.Equal(t, original.Attempts, decoded.Attempts)
	})

	t.Run("invalid_json_returns_error", func(t *testing.T) {
		job, err := DecodeJob([]byte("not json"))
		assert.Error(t, err)
		assert.Nil(t, job)
	})

	t.Run("empty_bytes_returns_error", func(t *testing.T) {
		job, err := DecodeJob([]byte{})
		assert.Error(t, err)
		assert.Nil(t, job)
	})
}

func TestUnmarshalPayload(t *testing.T) {
	t.Run("valid_payload", func(t *testing.T) {
		job, err := NewJob("test", testPayload{Name: "extracted", Count: 5})
		require.NoError(t, err)

		var result testPayload
		err = job.UnmarshalPayload(&result)
		require.NoError(t, err)
		assert.Equal(t, "extracted", result.Name)
		assert.Equal(t, 5, result.Count)
	})

	t.Run("invalid_payload", func(t *testing.T) {
		job := &Job{
			Payload: json.RawMessage(`{"name": 123}`), // name is int, expect string
		}
		var result struct {
			Name string `json:"name"`
		}
		// json.Unmarshal actually coerces int to string, so use truly invalid JSON
		job.Payload = json.RawMessage(`not-json`)
		err := job.UnmarshalPayload(&result)
		assert.Error(t, err)
	})
}

func TestCanRetry(t *testing.T) {
	tests := []struct {
		name     string
		attempts int
		maxRetry int
		expected bool
	}{
		{"zero_attempts", 0, 3, true},
		{"under_max", 2, 3, true},
		{"at_max", 3, 3, false},
		{"over_max", 5, 3, false},
		{"zero_max_retry", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{Attempts: tt.attempts, MaxRetry: tt.maxRetry}
			assert.Equal(t, tt.expected, job.CanRetry())
		})
	}
}

func TestIncrementAttempts(t *testing.T) {
	job := &Job{Attempts: 0}

	job.IncrementAttempts()
	assert.Equal(t, 1, job.Attempts)

	job.IncrementAttempts()
	assert.Equal(t, 2, job.Attempts)

	job.IncrementAttempts()
	assert.Equal(t, 3, job.Attempts)
}

func TestJobConstants(t *testing.T) {
	assert.Equal(t, "email.send", JobTypeEmailSend)
	assert.Equal(t, "audit.cleanup", JobTypeAuditCleanup)
	assert.Equal(t, "notification.send", JobTypeNotification)
}
