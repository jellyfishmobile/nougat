package main

import "time"

type JobStatus string

const (
	StatusPending   JobStatus = "PENDING"
	StatusRunning   JobStatus = "RUNNING"
	StatusCompleted JobStatus = "COMPLETED"
	StatusFailed    JobStatus = "FAILED"
	StatusTimeout   JobStatus = "TIMEOUT"
)

type TaskRequest struct {
	Prompt           string `json:"prompt"`
	WorkingDirectory string `json:"working_directory"`
	TaskType         string `json:"task_type"`
	Timeout          int    `json:"timeout"`
}

type JobResponse struct {
	JobID            string     `json:"job_id"`
	Status           JobStatus  `json:"status"`
	TaskType         string     `json:"task_type"`
	WorkingDirectory string     `json:"working_directory"`
	ExitCode         int        `json:"exit_code,omitempty"`
	Stdout           string     `json:"stdout,omitempty"`
	Stderr           string     `json:"stderr,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

// Job is the persisted and API-returned document; it includes the prompt for full state.
type Job struct {
	JobResponse
	Prompt string `json:"prompt,omitempty"`
}

// JobLog is written to logs/{job_id}.json after the task finishes.
type JobLog struct {
	Job
	Error string `json:"error,omitempty"`
}
