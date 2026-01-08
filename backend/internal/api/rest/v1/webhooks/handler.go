package webhooks

import (
	"context"
	"log/slog"

	"gorm.io/gorm"
)

// WebhookContext represents the context for webhook processing
type WebhookContext struct {
	Context    context.Context
	DB         *gorm.DB
	Payload    map[string]interface{}
	ObjectKind string
	ProjectID  string

	// Pipeline-specific
	PipelineID     int64
	PipelineStatus string

	// MR-specific
	MRIID int

	// Results from handlers
	Results map[string]interface{}
}

// NewWebhookContext creates a new webhook context
func NewWebhookContext(ctx context.Context, db *gorm.DB, payload map[string]interface{}) *WebhookContext {
	wc := &WebhookContext{
		Context: ctx,
		DB:      db,
		Payload: payload,
		Results: make(map[string]interface{}),
	}

	// Extract common fields
	if objectKind, ok := payload["object_kind"].(string); ok {
		wc.ObjectKind = objectKind
	}

	// Extract project info
	if project, ok := payload["project"].(map[string]interface{}); ok {
		if id, ok := project["id"].(float64); ok {
			wc.ProjectID = formatID(int64(id))
		}
	}

	// Extract pipeline info
	if objAttrs, ok := payload["object_attributes"].(map[string]interface{}); ok {
		if id, ok := objAttrs["id"].(float64); ok {
			wc.PipelineID = int64(id)
		}
		if status, ok := objAttrs["status"].(string); ok {
			wc.PipelineStatus = status
		}
		if iid, ok := objAttrs["iid"].(float64); ok {
			wc.MRIID = int(iid)
		}
	}

	return wc
}

// AddResult adds a handler result to the context
func (c *WebhookContext) AddResult(handlerName string, result interface{}) {
	c.Results[handlerName] = result
}

// GetResult gets a handler result from the context
func (c *WebhookContext) GetResult(handlerName string) (interface{}, bool) {
	result, ok := c.Results[handlerName]
	return result, ok
}

func formatID(id int64) string {
	return string(rune(id))
}

// Handler is the interface for webhook handlers
type Handler interface {
	// CanHandle checks if this handler can process the event
	CanHandle(ctx *WebhookContext) bool

	// Handle processes the webhook event
	Handle(ctx *WebhookContext) (map[string]interface{}, error)
}

// CompositeHandler executes multiple sub-handlers in sequence
type CompositeHandler struct {
	subHandlers []Handler
	logger      *slog.Logger
}

// NewCompositeHandler creates a new composite handler
func NewCompositeHandler(logger *slog.Logger) *CompositeHandler {
	return &CompositeHandler{
		subHandlers: []Handler{},
		logger:      logger,
	}
}

// AddSubHandler adds a sub-handler to the composite
func (h *CompositeHandler) AddSubHandler(handler Handler) {
	h.subHandlers = append(h.subHandlers, handler)
}

// CanHandle returns true (let sub-handlers decide)
func (h *CompositeHandler) CanHandle(ctx *WebhookContext) bool {
	return true
}

// Handle executes all sub-handlers in sequence
func (h *CompositeHandler) Handle(ctx *WebhookContext) (map[string]interface{}, error) {
	results := make(map[string]interface{})

	for _, handler := range h.subHandlers {
		handlerName := getHandlerName(handler)

		if !handler.CanHandle(ctx) {
			h.logger.Debug("sub-handler declined to handle", "handler", handlerName)
			continue
		}

		result, err := handler.Handle(ctx)
		if err != nil {
			h.logger.Error("sub-handler failed", "handler", handlerName, "error", err)
			results[handlerName] = map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			}
			ctx.AddResult(handlerName, results[handlerName])
			continue
		}

		results[handlerName] = result
		ctx.AddResult(handlerName, result)
	}

	return map[string]interface{}{
		"status":      "ok",
		"sub_results": results,
	}, nil
}

func getHandlerName(handler Handler) string {
	// Use type assertion to get the concrete type name
	return "handler"
}

// HandlerRegistry manages webhook handlers by event type
type HandlerRegistry struct {
	handlers map[string]Handler
	logger   *slog.Logger
}

// NewHandlerRegistry creates a new handler registry
func NewHandlerRegistry(logger *slog.Logger) *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]Handler),
		logger:   logger,
	}
}

// Register adds a handler for an event type
func (r *HandlerRegistry) Register(eventType string, handler Handler) {
	r.handlers[eventType] = handler
}

// GetHandler returns the handler for an event type
func (r *HandlerRegistry) GetHandler(eventType string) (Handler, bool) {
	handler, ok := r.handlers[eventType]
	return handler, ok
}

// Process dispatches the webhook to the appropriate handler
func (r *HandlerRegistry) Process(ctx *WebhookContext) (map[string]interface{}, error) {
	handler, ok := r.GetHandler(ctx.ObjectKind)
	if !ok {
		r.logger.Debug("no handler for event type", "type", ctx.ObjectKind)
		return map[string]interface{}{
			"status": "skipped",
			"reason": "no_handler",
		}, nil
	}

	if !handler.CanHandle(ctx) {
		r.logger.Debug("handler declined to process", "type", ctx.ObjectKind)
		return map[string]interface{}{
			"status": "skipped",
			"reason": "handler_declined",
		}, nil
	}

	return handler.Handle(ctx)
}
