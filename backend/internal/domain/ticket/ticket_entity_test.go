package ticket

import (
	"testing"
)

// --- Test Ticket ---

func TestTicketTableName(t *testing.T) {
	ticket := Ticket{}
	if ticket.TableName() != "tickets" {
		t.Errorf("expected 'tickets', got %s", ticket.TableName())
	}
}

func TestTicketIsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{"in_progress is active", TicketStatusInProgress, true},
		{"in_review is active", TicketStatusInReview, true},
		{"backlog not active", TicketStatusBacklog, false},
		{"todo not active", TicketStatusTodo, false},
		{"done not active", TicketStatusDone, false},
		{"cancelled not active", TicketStatusCancelled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &Ticket{Status: tt.status}
			if ticket.IsActive() != tt.expected {
				t.Errorf("expected IsActive() = %v, got %v", tt.expected, ticket.IsActive())
			}
		})
	}
}

func TestTicketIsCompleted(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{TicketStatusDone, true},
		{TicketStatusBacklog, false},
		{TicketStatusInProgress, false},
		{TicketStatusCancelled, false},
	}

	for _, tt := range tests {
		ticket := &Ticket{Status: tt.status}
		if ticket.IsCompleted() != tt.expected {
			t.Errorf("status %s: expected IsCompleted() = %v", tt.status, tt.expected)
		}
	}
}

func TestTicketIsCancelled(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{TicketStatusCancelled, true},
		{TicketStatusDone, false},
		{TicketStatusBacklog, false},
	}

	for _, tt := range tests {
		ticket := &Ticket{Status: tt.status}
		if ticket.IsCancelled() != tt.expected {
			t.Errorf("status %s: expected IsCancelled() = %v", tt.status, tt.expected)
		}
	}
}

func TestTicketIsBug(t *testing.T) {
	tests := []struct {
		ticketType string
		expected   bool
	}{
		{TicketTypeBug, true},
		{TicketTypeTask, false},
		{TicketTypeFeature, false},
		{TicketTypeEpic, false},
	}

	for _, tt := range tests {
		ticket := &Ticket{Type: tt.ticketType}
		if ticket.IsBug() != tt.expected {
			t.Errorf("type %s: expected IsBug() = %v", tt.ticketType, tt.expected)
		}
	}
}

func TestTicketHasSubTickets(t *testing.T) {
	ticketWithSubs := &Ticket{
		SubTickets: []Ticket{{ID: 1}, {ID: 2}},
	}
	if !ticketWithSubs.HasSubTickets() {
		t.Error("expected HasSubTickets() = true")
	}

	ticketWithoutSubs := &Ticket{}
	if ticketWithoutSubs.HasSubTickets() {
		t.Error("expected HasSubTickets() = false")
	}
}
