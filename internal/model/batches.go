package model

// BatchRequest represents a request to create a batch.
type BatchRequest struct {
	InputFileID      string         `json:"input_file_id"`
	Endpoint         string         `json:"endpoint"`
	CompletionWindow string         `json:"completion_window"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// BatchObject represents an OpenAI batch object.
type BatchObject struct {
	ID               string         `json:"id"`
	Object           string         `json:"object"`
	Endpoint         string         `json:"endpoint"`
	Errors           *BatchErrors   `json:"errors,omitempty"`
	InputFileID      string         `json:"input_file_id"`
	CompletionWindow string         `json:"completion_window"`
	Status           string         `json:"status"`
	OutputFileID     *string        `json:"output_file_id,omitempty"`
	ErrorFileID      *string        `json:"error_file_id,omitempty"`
	CreatedAt        int64          `json:"created_at"`
	InProgressAt     *int64         `json:"in_progress_at,omitempty"`
	ExpiresAt        *int64         `json:"expires_at,omitempty"`
	FinalizingAt     *int64         `json:"finalizing_at,omitempty"`
	CompletedAt      *int64         `json:"completed_at,omitempty"`
	FailedAt         *int64         `json:"failed_at,omitempty"`
	ExpiredAt        *int64         `json:"expired_at,omitempty"`
	CancellingAt     *int64         `json:"cancelling_at,omitempty"`
	CancelledAt      *int64         `json:"cancelled_at,omitempty"`
	RequestCounts    *RequestCounts `json:"request_counts,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// BatchErrors holds batch error details.
type BatchErrors struct {
	Object string       `json:"object"`
	Data   []BatchError `json:"data"`
}

// BatchError represents a single batch error.
type BatchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
	Line    *int   `json:"line,omitempty"`
}

// RequestCounts holds batch request counts.
type RequestCounts struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// BatchListResponse represents the response for listing batches.
type BatchListResponse struct {
	Data    []BatchObject `json:"data"`
	Object  string        `json:"object"`
	FirstID *string       `json:"first_id,omitempty"`
	LastID  *string       `json:"last_id,omitempty"`
	HasMore bool          `json:"has_more"`
}
