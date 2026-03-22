package handler

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/14mdzk/goscratch/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// Handler handles SSE HTTP requests
type Handler struct {
	broker port.SSEBroker
}

// NewHandler creates a new SSE handler
func NewHandler(broker port.SSEBroker) *Handler {
	return &Handler{broker: broker}
}

// BroadcastRequest represents a broadcast request body
type BroadcastRequest struct {
	Event string `json:"event"`
	Data  string `json:"data"`
	Topic string `json:"topic,omitempty"`
}

// Subscribe handles the SSE stream endpoint
func (h *Handler) Subscribe(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return response.Unauthorized(c, "Authentication required")
	}

	// Parse optional topics
	var topics []string
	if topicsParam := c.Query("topics"); topicsParam != "" {
		for _, t := range strings.Split(topicsParam, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				topics = append(topics, t)
			}
		}
	}

	// Subscribe to broker
	ch := h.broker.Subscribe(userID, topics...)

	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		for event := range ch {
			// Write SSE formatted event
			if event.ID != "" {
				fmt.Fprintf(w, "id: %s\n", event.ID)
			}
			if event.Event != "" {
				fmt.Fprintf(w, "event: %s\n", event.Event)
			}
			if len(event.Data) > 0 {
				fmt.Fprintf(w, "data: %s\n", string(event.Data))
			}
			if event.Retry > 0 {
				fmt.Fprintf(w, "retry: %d\n", event.Retry)
			}
			fmt.Fprint(w, "\n")

			// Flush to send the event immediately
			if err := w.Flush(); err != nil {
				// Client disconnected
				break
			}
		}

		// Unsubscribe when stream ends (client disconnected or channel closed)
		h.broker.Unsubscribe(userID)
	})

	return nil
}

// Broadcast handles broadcasting an event to all clients or a specific topic
func (h *Handler) Broadcast(c *fiber.Ctx) error {
	var req BroadcastRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, apperr.BadRequestf("Invalid request body"))
	}

	if req.Event == "" {
		return response.Fail(c, apperr.BadRequestf("Event type is required"))
	}

	event := port.NewEvent(req.Event, []byte(req.Data))

	if req.Topic != "" {
		h.broker.BroadcastToTopic(req.Topic, event)
	} else {
		h.broker.Broadcast(event)
	}

	return response.Message(c, "Event broadcast successfully")
}

// ClientCount returns the number of connected SSE clients
func (h *Handler) ClientCount(c *fiber.Ctx) error {
	count := h.broker.ClientCount()
	return response.Success(c, fiber.Map{
		"count": count,
	})
}
