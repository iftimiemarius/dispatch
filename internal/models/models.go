// Package models defines the core domain entities for Dispatch.
//
// The data model is a flexible hierarchy where every link is optional:
//
//	Initiative → Project → Task → Block
//
// A captured task starts in the "inbox" with no links. It may later be
// attached to a project, an initiative, both, or neither. A block is a
// time reservation on the calendar, optionally tied to a task.
package models

import "time"

// TaskStatus is the lifecycle state of a task.
type TaskStatus string

const (
	// StatusInbox is the default state for freshly captured tasks awaiting triage.
	StatusInbox TaskStatus = "inbox"
	// StatusTodo is a triaged task ready to be worked on.
	StatusTodo TaskStatus = "todo"
	// StatusDoing is a task actively in progress.
	StatusDoing TaskStatus = "doing"
	// StatusDone is a completed task.
	StatusDone TaskStatus = "done"
	// StatusBlocked is a task that cannot proceed pending an external dependency.
	StatusBlocked TaskStatus = "blocked"
	// StatusCancelled is a task that was abandoned.
	StatusCancelled TaskStatus = "cancelled"
)

// Priority orders work. Higher value = more urgent.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// Task is the atomic unit of work.
type Task struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Notes        string    `json:"notes"`
	Status       TaskStatus `json:"status"`
	Priority     Priority  `json:"priority"`
	ProjectID    *string   `json:"project_id,omitempty"`
	InitiativeID *string   `json:"initiative_id,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	DueAt        *time.Time `json:"due_at,omitempty"`
	GitHubRepo   *string   `json:"github_repo,omitempty"`
	GitHubIssue  *int      `json:"github_issue_number,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// Project groups tasks by what is being built.
type Project struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Color       string     `json:"color,omitempty"`
	InitiativeID *string   `json:"initiative_id,omitempty"`
	GitHubRepo  *string    `json:"github_repo,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Initiative groups projects and tasks by the strategic outcome they drive.
type Initiative struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Outcome   string     `json:"outcome"`
	Status    string     `json:"status"`
	StartAt   *time.Time `json:"start_at,omitempty"`
	TargetAt  *time.Time `json:"target_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Block is a time reservation on the calendar for a task.
type Block struct {
	ID             string     `json:"id"`
	TaskID         *string    `json:"task_id,omitempty"`
	Title          string     `json:"title"`
	Notes          string     `json:"notes"`
	StartsAt       time.Time  `json:"starts_at"`
	EndsAt         time.Time  `json:"ends_at"`
	OutlookEventID *string    `json:"outlook_event_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
