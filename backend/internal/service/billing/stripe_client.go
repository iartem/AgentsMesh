package billing

import (
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
)

// StripeClient defines the interface for Stripe API operations
// This allows us to mock Stripe calls in tests
type StripeClient interface {
	// CreateCustomer creates a new Stripe customer
	CreateCustomer(params *stripe.CustomerParams) (*stripe.Customer, error)

	// CancelSubscription cancels a Stripe subscription
	CancelSubscription(id string, params *stripe.SubscriptionCancelParams) (*stripe.Subscription, error)
}

// DefaultStripeClient is the real Stripe client implementation
type DefaultStripeClient struct{}

// NewDefaultStripeClient creates a new default Stripe client
func NewDefaultStripeClient() *DefaultStripeClient {
	return &DefaultStripeClient{}
}

// CreateCustomer creates a new Stripe customer using the Stripe SDK
func (c *DefaultStripeClient) CreateCustomer(params *stripe.CustomerParams) (*stripe.Customer, error) {
	return customer.New(params)
}

// CancelSubscription cancels a Stripe subscription using the Stripe SDK
func (c *DefaultStripeClient) CancelSubscription(id string, params *stripe.SubscriptionCancelParams) (*stripe.Subscription, error) {
	return subscription.Cancel(id, params)
}
