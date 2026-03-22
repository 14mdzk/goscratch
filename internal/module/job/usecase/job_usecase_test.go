package usecase

import (
	"context"
	"testing"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockQueue implements port.Queue for testing
type MockQueue struct {
	mock.Mock
}

func (m *MockQueue) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	args := m.Called(ctx, exchange, routingKey, body)
	return args.Error(0)
}

func (m *MockQueue) PublishJSON(ctx context.Context, exchange, routingKey string, message any) error {
	args := m.Called(ctx, exchange, routingKey, message)
	return args.Error(0)
}

func (m *MockQueue) Consume(ctx context.Context, queue string, handler func(body []byte) error) error {
	args := m.Called(ctx, queue, handler)
	return args.Error(0)
}

func (m *MockQueue) DeclareQueue(ctx context.Context, name string, durable bool) error {
	args := m.Called(ctx, name, durable)
	return args.Error(0)
}

func (m *MockQueue) DeclareExchange(ctx context.Context, name, kind string, durable bool) error {
	args := m.Called(ctx, name, kind, durable)
	return args.Error(0)
}

func (m *MockQueue) BindQueue(ctx context.Context, queue, exchange, routingKey string) error {
	args := m.Called(ctx, queue, exchange, routingKey)
	return args.Error(0)
}

func (m *MockQueue) Close() error {
	return nil
}

// Ensure MockQueue implements port.Queue
var _ port.Queue = (*MockQueue)(nil)

func TestUseCase_Dispatch(t *testing.T) {
	ctx := context.Background()

	t.Run("success_email_send", func(t *testing.T) {
		mockQueue := new(MockQueue)
		mockQueue.On("Publish", ctx, "", "jobs", mock.AnythingOfType("[]uint8")).Return(nil)

		publisher := worker.NewPublisher(mockQueue, "jobs", "")
		uc := NewUseCase(publisher)

		payload := map[string]string{"to": "user@example.com", "subject": "Hello"}
		result, err := uc.Dispatch(ctx, "email.send", payload, 3)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "email.send", result.Type)
		assert.Equal(t, "queued", result.Status)
		assert.NotEmpty(t, result.ID)
		assert.NotEmpty(t, result.CreatedAt)
		mockQueue.AssertExpectations(t)
	})

	t.Run("success_audit_cleanup", func(t *testing.T) {
		mockQueue := new(MockQueue)
		mockQueue.On("Publish", ctx, "", "jobs", mock.AnythingOfType("[]uint8")).Return(nil)

		publisher := worker.NewPublisher(mockQueue, "jobs", "")
		uc := NewUseCase(publisher)

		payload := map[string]int{"older_than_days": 90}
		result, err := uc.Dispatch(ctx, "audit.cleanup", payload, 1)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "audit.cleanup", result.Type)
		mockQueue.AssertExpectations(t)
	})

	t.Run("success_notification", func(t *testing.T) {
		mockQueue := new(MockQueue)
		mockQueue.On("Publish", ctx, "", "jobs", mock.AnythingOfType("[]uint8")).Return(nil)

		publisher := worker.NewPublisher(mockQueue, "jobs", "")
		uc := NewUseCase(publisher)

		payload := map[string]string{"user_id": "123", "message": "Hello"}
		result, err := uc.Dispatch(ctx, "notification.send", payload, 5)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "notification.send", result.Type)
		mockQueue.AssertExpectations(t)
	})

	t.Run("invalid_job_type", func(t *testing.T) {
		mockQueue := new(MockQueue)
		publisher := worker.NewPublisher(mockQueue, "jobs", "")
		uc := NewUseCase(publisher)

		payload := map[string]string{"foo": "bar"}
		result, err := uc.Dispatch(ctx, "invalid.type", payload, 3)

		assert.Nil(t, result)
		assert.Error(t, err)

		appErr, ok := apperr.AsAppError(err)
		assert.True(t, ok)
		assert.Equal(t, apperr.CodeBadRequest, appErr.Code)
		mockQueue.AssertNotCalled(t, "Publish")
	})

	t.Run("publish_failure", func(t *testing.T) {
		mockQueue := new(MockQueue)
		mockQueue.On("Publish", ctx, "", "jobs", mock.AnythingOfType("[]uint8")).Return(assert.AnError)

		publisher := worker.NewPublisher(mockQueue, "jobs", "")
		uc := NewUseCase(publisher)

		payload := map[string]string{"to": "user@example.com"}
		result, err := uc.Dispatch(ctx, "email.send", payload, 3)

		assert.Nil(t, result)
		assert.Error(t, err)

		appErr, ok := apperr.AsAppError(err)
		assert.True(t, ok)
		assert.Equal(t, apperr.CodeInternalError, appErr.Code)
		mockQueue.AssertExpectations(t)
	})
}

func TestUseCase_ListJobTypes(t *testing.T) {
	ctx := context.Background()

	t.Run("returns_all_types", func(t *testing.T) {
		mockQueue := new(MockQueue)
		publisher := worker.NewPublisher(mockQueue, "jobs", "")
		uc := NewUseCase(publisher)

		result := uc.ListJobTypes(ctx)

		assert.NotNil(t, result)
		assert.Len(t, result.Types, 3)

		// Collect types
		typeMap := make(map[string]string)
		for _, jt := range result.Types {
			typeMap[jt.Type] = jt.Description
		}

		assert.Contains(t, typeMap, "email.send")
		assert.Contains(t, typeMap, "audit.cleanup")
		assert.Contains(t, typeMap, "notification.send")

		// Verify descriptions are not empty
		for _, desc := range typeMap {
			assert.NotEmpty(t, desc)
		}
	})
}
