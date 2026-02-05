package mcp

import (
	"context"
	"net/http"
	"net/url"

	"github.com/anthropics/agentsmesh/runner/internal/mcp/tools"
)

// Binding Operations

// RequestBinding requests a binding with another pod.
func (c *BackendClient) RequestBinding(ctx context.Context, targetPod string, scopes []tools.BindingScope) (*tools.Binding, error) {
	body := map[string]interface{}{
		"target_pod": targetPod,
		"scopes":     scopes,
	}

	var result struct {
		Binding tools.Binding `json:"binding"`
	}
	err := c.request(ctx, http.MethodPost, c.podAPIPath()+"/bindings", body, &result)
	if err != nil {
		return nil, err
	}
	return &result.Binding, nil
}

// AcceptBinding accepts a binding request.
func (c *BackendClient) AcceptBinding(ctx context.Context, bindingID int) (*tools.Binding, error) {
	body := map[string]interface{}{
		"binding_id": bindingID,
	}

	var result struct {
		Binding tools.Binding `json:"binding"`
	}
	err := c.request(ctx, http.MethodPost, c.podAPIPath()+"/bindings/accept", body, &result)
	if err != nil {
		return nil, err
	}
	return &result.Binding, nil
}

// RejectBinding rejects a binding request.
func (c *BackendClient) RejectBinding(ctx context.Context, bindingID int, reason string) (*tools.Binding, error) {
	body := map[string]interface{}{
		"binding_id": bindingID,
		"reason":     reason,
	}

	var result struct {
		Binding tools.Binding `json:"binding"`
	}
	err := c.request(ctx, http.MethodPost, c.podAPIPath()+"/bindings/reject", body, &result)
	if err != nil {
		return nil, err
	}
	return &result.Binding, nil
}

// UnbindPod unbinds from another pod.
func (c *BackendClient) UnbindPod(ctx context.Context, targetPod string) error {
	body := map[string]interface{}{
		"target_pod": targetPod,
	}
	return c.request(ctx, http.MethodPost, c.podAPIPath()+"/bindings/unbind", body, nil)
}

// GetBindings gets all bindings for the current pod.
func (c *BackendClient) GetBindings(ctx context.Context, status *tools.BindingStatus) ([]tools.Binding, error) {
	path := c.podAPIPath() + "/bindings"
	if status != nil {
		path += "?status=" + url.QueryEscape(string(*status))
	}

	var result struct {
		Bindings []tools.Binding `json:"bindings"`
	}
	err := c.request(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Bindings, nil
}

// GetBoundPods gets pods that are bound to the current pod.
// Returns a list of pod keys (strings) that this pod has active bindings with.
func (c *BackendClient) GetBoundPods(ctx context.Context) ([]string, error) {
	var result struct {
		Pods []string `json:"pods"`
	}
	err := c.request(ctx, http.MethodGet, c.podAPIPath()+"/bindings/pods", nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Pods, nil
}
