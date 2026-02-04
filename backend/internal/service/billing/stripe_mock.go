package billing

import (
	"fmt"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v76"
)

// MockStripeClient is a mock implementation of StripeClient for testing
type MockStripeClient struct {
	mu sync.RWMutex

	// Store mock data
	customers     map[string]*stripe.Customer
	subscriptions map[string]*stripe.Subscription

	// Configurable error responses
	CreateCustomerErr       error
	CancelSubscriptionErr   error

	// Call tracking for verification
	CreateCustomerCalls     []CreateCustomerCall
	CancelSubscriptionCalls []CancelSubscriptionCall

	// Auto-increment ID counter
	idCounter int64
}

// CreateCustomerCall records a CreateCustomer call
type CreateCustomerCall struct {
	Params *stripe.CustomerParams
	Result *stripe.Customer
	Error  error
}

// CancelSubscriptionCall records a CancelSubscription call
type CancelSubscriptionCall struct {
	ID     string
	Params *stripe.SubscriptionCancelParams
	Result *stripe.Subscription
	Error  error
}

// NewMockStripeClient creates a new mock Stripe client
func NewMockStripeClient() *MockStripeClient {
	return &MockStripeClient{
		customers:     make(map[string]*stripe.Customer),
		subscriptions: make(map[string]*stripe.Subscription),
	}
}

// generateID generates a mock Stripe-like ID
func (m *MockStripeClient) generateID(prefix string) string {
	m.idCounter++
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), m.idCounter)
}

// CreateCustomer creates a mock Stripe customer
func (m *MockStripeClient) CreateCustomer(params *stripe.CustomerParams) (*stripe.Customer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return configured error if set
	if m.CreateCustomerErr != nil {
		call := CreateCustomerCall{Params: params, Error: m.CreateCustomerErr}
		m.CreateCustomerCalls = append(m.CreateCustomerCalls, call)
		return nil, m.CreateCustomerErr
	}

	// Create mock customer
	customerID := m.generateID("cus")
	cust := &stripe.Customer{
		ID:      customerID,
		Email:   stripe.StringValue(params.Email),
		Name:    stripe.StringValue(params.Name),
		Created: time.Now().Unix(),
		Livemode: false,
	}

	// Copy metadata if provided
	if params.Metadata != nil {
		cust.Metadata = params.Metadata
	}

	m.customers[customerID] = cust

	call := CreateCustomerCall{Params: params, Result: cust}
	m.CreateCustomerCalls = append(m.CreateCustomerCalls, call)

	return cust, nil
}

// CancelSubscription cancels a mock Stripe subscription
func (m *MockStripeClient) CancelSubscription(id string, params *stripe.SubscriptionCancelParams) (*stripe.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return configured error if set
	if m.CancelSubscriptionErr != nil {
		call := CancelSubscriptionCall{ID: id, Params: params, Error: m.CancelSubscriptionErr}
		m.CancelSubscriptionCalls = append(m.CancelSubscriptionCalls, call)
		return nil, m.CancelSubscriptionErr
	}

	// Get or create subscription
	sub, ok := m.subscriptions[id]
	if !ok {
		// Create a mock subscription if it doesn't exist
		sub = &stripe.Subscription{
			ID:       id,
			Status:   stripe.SubscriptionStatusActive,
			Created:  time.Now().Unix(),
			Livemode: false,
		}
		m.subscriptions[id] = sub
	}

	// Update status to canceled
	sub.Status = stripe.SubscriptionStatusCanceled
	sub.CanceledAt = time.Now().Unix()
	canceledAtTime := time.Now()
	sub.EndedAt = canceledAtTime.Unix()

	call := CancelSubscriptionCall{ID: id, Params: params, Result: sub}
	m.CancelSubscriptionCalls = append(m.CancelSubscriptionCalls, call)

	return sub, nil
}

// AddSubscription adds a mock subscription for testing
func (m *MockStripeClient) AddSubscription(sub *stripe.Subscription) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptions[sub.ID] = sub
}

// GetCustomer returns a mock customer by ID
func (m *MockStripeClient) GetCustomer(id string) (*stripe.Customer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cust, ok := m.customers[id]
	return cust, ok
}

// GetSubscription returns a mock subscription by ID
func (m *MockStripeClient) GetSubscription(id string) (*stripe.Subscription, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sub, ok := m.subscriptions[id]
	return sub, ok
}

// Reset clears all mock data and errors
func (m *MockStripeClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customers = make(map[string]*stripe.Customer)
	m.subscriptions = make(map[string]*stripe.Subscription)
	m.CreateCustomerErr = nil
	m.CancelSubscriptionErr = nil
	m.CreateCustomerCalls = nil
	m.CancelSubscriptionCalls = nil
	m.idCounter = 0
}
