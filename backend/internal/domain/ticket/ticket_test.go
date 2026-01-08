package ticket

import (
	"testing"
	"time"
)

// --- Test Constants ---

func TestTicketTypeConstants(t *testing.T) {
	if TicketTypeTask != "task" {
		t.Errorf("expected 'task', got %s", TicketTypeTask)
	}
	if TicketTypeBug != "bug" {
		t.Errorf("expected 'bug', got %s", TicketTypeBug)
	}
	if TicketTypeFeature != "feature" {
		t.Errorf("expected 'feature', got %s", TicketTypeFeature)
	}
	if TicketTypeEpic != "epic" {
		t.Errorf("expected 'epic', got %s", TicketTypeEpic)
	}
}

func TestTicketStatusConstants(t *testing.T) {
	if TicketStatusBacklog != "backlog" {
		t.Errorf("expected 'backlog', got %s", TicketStatusBacklog)
	}
	if TicketStatusTodo != "todo" {
		t.Errorf("expected 'todo', got %s", TicketStatusTodo)
	}
	if TicketStatusInProgress != "in_progress" {
		t.Errorf("expected 'in_progress', got %s", TicketStatusInProgress)
	}
	if TicketStatusInReview != "in_review" {
		t.Errorf("expected 'in_review', got %s", TicketStatusInReview)
	}
	if TicketStatusDone != "done" {
		t.Errorf("expected 'done', got %s", TicketStatusDone)
	}
	if TicketStatusCancelled != "cancelled" {
		t.Errorf("expected 'cancelled', got %s", TicketStatusCancelled)
	}
}

func TestTicketPriorityConstants(t *testing.T) {
	priorities := []string{TicketPriorityNone, TicketPriorityLow, TicketPriorityMedium, TicketPriorityHigh, TicketPriorityUrgent}
	expected := []string{"none", "low", "medium", "high", "urgent"}

	for i, p := range priorities {
		if p != expected[i] {
			t.Errorf("expected '%s', got '%s'", expected[i], p)
		}
	}
}

func TestTicketSeverityConstants(t *testing.T) {
	severities := []string{TicketSeverityCritical, TicketSeverityMajor, TicketSeverityMinor, TicketSeverityTrivial}
	expected := []string{"critical", "major", "minor", "trivial"}

	for i, s := range severities {
		if s != expected[i] {
			t.Errorf("expected '%s', got '%s'", expected[i], s)
		}
	}
}

func TestValidEstimates(t *testing.T) {
	expected := []int{1, 2, 3, 5, 8, 13, 21}
	if len(ValidEstimates) != len(expected) {
		t.Errorf("expected %d estimates, got %d", len(expected), len(ValidEstimates))
	}
	for i, v := range expected {
		if ValidEstimates[i] != v {
			t.Errorf("expected estimate[%d] = %d, got %d", i, v, ValidEstimates[i])
		}
	}
}

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

func TestIsValidEstimate(t *testing.T) {
	validEstimates := []int{1, 2, 3, 5, 8, 13, 21}
	for _, v := range validEstimates {
		if !IsValidEstimate(v) {
			t.Errorf("expected %d to be valid", v)
		}
	}

	invalidEstimates := []int{0, 4, 6, 7, 10, 100}
	for _, v := range invalidEstimates {
		if IsValidEstimate(v) {
			t.Errorf("expected %d to be invalid", v)
		}
	}
}

// --- Test Assignee ---

func TestAssigneeTableName(t *testing.T) {
	a := Assignee{}
	if a.TableName() != "ticket_assignees" {
		t.Errorf("expected 'ticket_assignees', got %s", a.TableName())
	}
}

func TestAssigneeStruct(t *testing.T) {
	a := Assignee{TicketID: 1, UserID: 100}
	if a.TicketID != 1 {
		t.Errorf("expected TicketID 1, got %d", a.TicketID)
	}
	if a.UserID != 100 {
		t.Errorf("expected UserID 100, got %d", a.UserID)
	}
}

// --- Test Label ---

func TestLabelTableName(t *testing.T) {
	l := Label{}
	if l.TableName() != "labels" {
		t.Errorf("expected 'labels', got %s", l.TableName())
	}
}

func TestLabelStruct(t *testing.T) {
	repoID := int64(10)
	l := Label{
		ID:             1,
		OrganizationID: 100,
		RepositoryID:   &repoID,
		Name:           "bug",
		Color:          "#FF0000",
	}

	if l.Name != "bug" {
		t.Errorf("expected Name 'bug', got %s", l.Name)
	}
	if l.Color != "#FF0000" {
		t.Errorf("expected Color '#FF0000', got %s", l.Color)
	}
}

// --- Test TicketLabel ---

func TestTicketLabelTableName(t *testing.T) {
	tl := TicketLabel{}
	if tl.TableName() != "ticket_labels" {
		t.Errorf("expected 'ticket_labels', got %s", tl.TableName())
	}
}

// --- Test MR Constants ---

func TestMRStateConstants(t *testing.T) {
	if MRStateOpened != "opened" {
		t.Errorf("expected 'opened', got %s", MRStateOpened)
	}
	if MRStateMerged != "merged" {
		t.Errorf("expected 'merged', got %s", MRStateMerged)
	}
	if MRStateClosed != "closed" {
		t.Errorf("expected 'closed', got %s", MRStateClosed)
	}
}

func TestPipelineStatusConstants(t *testing.T) {
	statuses := []string{
		PipelineStatusPending, PipelineStatusRunning, PipelineStatusSuccess,
		PipelineStatusFailed, PipelineStatusCanceled, PipelineStatusSkipped, PipelineStatusManual,
	}
	expected := []string{"pending", "running", "success", "failed", "canceled", "skipped", "manual"}

	for i, s := range statuses {
		if s != expected[i] {
			t.Errorf("expected '%s', got '%s'", expected[i], s)
		}
	}
}

// --- Test MergeRequest ---

func TestMergeRequestTableName(t *testing.T) {
	mr := MergeRequest{}
	if mr.TableName() != "ticket_merge_requests" {
		t.Errorf("expected 'ticket_merge_requests', got %s", mr.TableName())
	}
}

func TestMergeRequestIsMerged(t *testing.T) {
	tests := []struct {
		state    string
		expected bool
	}{
		{MRStateMerged, true},
		{MRStateOpened, false},
		{MRStateClosed, false},
	}

	for _, tt := range tests {
		mr := &MergeRequest{State: tt.state}
		if mr.IsMerged() != tt.expected {
			t.Errorf("state %s: expected IsMerged() = %v", tt.state, tt.expected)
		}
	}
}

func TestMergeRequestIsOpen(t *testing.T) {
	tests := []struct {
		state    string
		expected bool
	}{
		{MRStateOpened, true},
		{MRStateMerged, false},
		{MRStateClosed, false},
	}

	for _, tt := range tests {
		mr := &MergeRequest{State: tt.state}
		if mr.IsOpen() != tt.expected {
			t.Errorf("state %s: expected IsOpen() = %v", tt.state, tt.expected)
		}
	}
}

func TestMergeRequestHasPipeline(t *testing.T) {
	status := PipelineStatusRunning
	mrWithPipeline := &MergeRequest{PipelineStatus: &status}
	if !mrWithPipeline.HasPipeline() {
		t.Error("expected HasPipeline() = true")
	}

	mrWithoutPipeline := &MergeRequest{}
	if mrWithoutPipeline.HasPipeline() {
		t.Error("expected HasPipeline() = false")
	}
}

func TestMergeRequestIsPipelineSuccess(t *testing.T) {
	success := PipelineStatusSuccess
	failed := PipelineStatusFailed

	tests := []struct {
		name     string
		status   *string
		expected bool
	}{
		{"success", &success, true},
		{"failed", &failed, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := &MergeRequest{PipelineStatus: tt.status}
			if mr.IsPipelineSuccess() != tt.expected {
				t.Errorf("expected IsPipelineSuccess() = %v", tt.expected)
			}
		})
	}
}

// --- Test Relation ---

func TestRelationTypeConstants(t *testing.T) {
	if RelationTypeBlocks != "blocks" {
		t.Errorf("expected 'blocks', got %s", RelationTypeBlocks)
	}
	if RelationTypeBlockedBy != "blocked_by" {
		t.Errorf("expected 'blocked_by', got %s", RelationTypeBlockedBy)
	}
	if RelationTypeRelates != "relates_to" {
		t.Errorf("expected 'relates_to', got %s", RelationTypeRelates)
	}
	if RelationTypeDuplicate != "duplicates" {
		t.Errorf("expected 'duplicates', got %s", RelationTypeDuplicate)
	}
}

func TestRelationTableName(t *testing.T) {
	r := Relation{}
	if r.TableName() != "ticket_relations" {
		t.Errorf("expected 'ticket_relations', got %s", r.TableName())
	}
}

// --- Test Commit ---

func TestCommitTableName(t *testing.T) {
	c := Commit{}
	if c.TableName() != "ticket_commits" {
		t.Errorf("expected 'ticket_commits', got %s", c.TableName())
	}
}

func TestCommitStruct(t *testing.T) {
	now := time.Now()
	url := "https://github.com/org/repo/commit/abc123"
	author := "Test User"
	email := "test@example.com"

	c := Commit{
		ID:             1,
		OrganizationID: 100,
		TicketID:       10,
		RepositoryID:   5,
		CommitSHA:      "abc123def456",
		CommitMessage:  "Fix bug",
		CommitURL:      &url,
		AuthorName:     &author,
		AuthorEmail:    &email,
		CommittedAt:    &now,
		CreatedAt:      now,
	}

	if c.CommitSHA != "abc123def456" {
		t.Errorf("expected CommitSHA 'abc123def456', got %s", c.CommitSHA)
	}
	if *c.AuthorName != "Test User" {
		t.Errorf("expected AuthorName 'Test User', got %s", *c.AuthorName)
	}
}

// --- Test Board ---

func TestBoardColumnStruct(t *testing.T) {
	col := BoardColumn{
		Status:  TicketStatusInProgress,
		Count:   5,
		Tickets: []Ticket{{ID: 1}, {ID: 2}},
	}

	if col.Status != TicketStatusInProgress {
		t.Errorf("expected Status 'in_progress', got %s", col.Status)
	}
	if col.Count != 5 {
		t.Errorf("expected Count 5, got %d", col.Count)
	}
	if len(col.Tickets) != 2 {
		t.Errorf("expected 2 tickets, got %d", len(col.Tickets))
	}
}

func TestBoardStruct(t *testing.T) {
	board := Board{
		Columns: []BoardColumn{
			{Status: TicketStatusBacklog, Count: 3},
			{Status: TicketStatusInProgress, Count: 2},
		},
	}

	if len(board.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(board.Columns))
	}
}

// --- Benchmark Tests ---

func BenchmarkTicketIsActive(b *testing.B) {
	ticket := &Ticket{Status: TicketStatusInProgress}
	for i := 0; i < b.N; i++ {
		ticket.IsActive()
	}
}

func BenchmarkIsValidEstimate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IsValidEstimate(5)
	}
}

func BenchmarkMergeRequestIsPipelineSuccess(b *testing.B) {
	status := PipelineStatusSuccess
	mr := &MergeRequest{PipelineStatus: &status}
	for i := 0; i < b.N; i++ {
		mr.IsPipelineSuccess()
	}
}
