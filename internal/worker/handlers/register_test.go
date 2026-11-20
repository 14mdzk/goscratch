package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/worker"
)

func TestRegisterDefault(t *testing.T) {
	log := newTestLogger()
	w := worker.New(queue.NewNoOpQueue(), log, worker.Config{
		QueueName:   "jobs",
		Concurrency: 1,
	})

	RegisterDefault(w, Dependencies{
		Logger:       log,
		DB:           nil,
		Email:        &mockEmailSender{},
		Notification: &mockNotificationSender{},
	})

	registered, ok := w.Stats()["handlers"].([]string)
	assert.True(t, ok)
	assert.ElementsMatch(t, []string{
		worker.JobTypeEmailSend,
		worker.JobTypeNotification,
		worker.JobTypeAuditCleanup,
	}, registered)
}
