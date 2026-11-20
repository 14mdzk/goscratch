package handlers

import (
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// Dependencies holds the shared dependencies required by the built-in job handlers.
type Dependencies struct {
	Logger       *logger.Logger
	DB           BatchDeleter
	Email        port.EmailSender
	Notification port.NotificationSender
}

// RegisterDefault registers all built-in job handlers on w.
// Both the standalone worker binary (cmd/worker) and the in-process API worker
// (internal/platform/app) call this so both paths execute the exact same set
// of job types.
func RegisterDefault(w *worker.Worker, deps Dependencies) {
	w.RegisterHandler(NewEmailHandler(deps.Logger, deps.Email))
	w.RegisterHandler(NewNotificationHandler(deps.Logger, deps.Notification))
	w.RegisterHandler(NewAuditCleanupHandler(deps.DB, deps.Logger))
}
