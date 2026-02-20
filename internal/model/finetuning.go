package model

// FineTuningRequest represents a request to create a fine-tuning job.
type FineTuningRequest struct {
	Model           string           `json:"model"`
	TrainingFile    string           `json:"training_file"`
	ValidationFile  *string          `json:"validation_file,omitempty"`
	Hyperparameters *Hyperparameters `json:"hyperparameters,omitempty"`
	Suffix          *string          `json:"suffix,omitempty"`
	Seed            *int             `json:"seed,omitempty"`
}

// Hyperparameters holds fine-tuning hyperparameters.
type Hyperparameters struct {
	NEpochs                any `json:"n_epochs,omitempty"`
	BatchSize              any `json:"batch_size,omitempty"`
	LearningRateMultiplier any `json:"learning_rate_multiplier,omitempty"`
}

// FineTuningJob represents an OpenAI fine-tuning job.
type FineTuningJob struct {
	ID              string           `json:"id"`
	Object          string           `json:"object"`
	CreatedAt       int64            `json:"created_at"`
	FinishedAt      *int64           `json:"finished_at,omitempty"`
	Model           string           `json:"model"`
	FineTunedModel  *string          `json:"fine_tuned_model,omitempty"`
	OrganizationID  string           `json:"organization_id"`
	Status          string           `json:"status"`
	Hyperparameters *Hyperparameters `json:"hyperparameters,omitempty"`
	TrainingFile    string           `json:"training_file"`
	ValidationFile  *string          `json:"validation_file,omitempty"`
	ResultFiles     []string         `json:"result_files"`
	TrainedTokens   *int64           `json:"trained_tokens,omitempty"`
	Error           *FineTuningError `json:"error,omitempty"`
	Seed            *int             `json:"seed,omitempty"`
}

// FineTuningError represents a fine-tuning job error.
type FineTuningError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
}

// FineTuningEvent represents a fine-tuning event.
type FineTuningEvent struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	CreatedAt int64  `json:"created_at"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// FineTuningCheckpoint represents a fine-tuning checkpoint.
type FineTuningCheckpoint struct {
	ID                       string                       `json:"id"`
	Object                   string                       `json:"object"`
	CreatedAt                int64                        `json:"created_at"`
	FineTuningJobID          string                       `json:"fine_tuning_job_id"`
	StepNumber               int                          `json:"step_number"`
	Metrics                  *FineTuningCheckpointMetrics `json:"metrics,omitempty"`
	FineTunedModelCheckpoint string                       `json:"fine_tuned_model_checkpoint"`
}

// FineTuningCheckpointMetrics holds checkpoint training metrics.
type FineTuningCheckpointMetrics struct {
	Step                       int     `json:"step"`
	TrainLoss                  float64 `json:"train_loss"`
	TrainMeanTokenAccuracy     float64 `json:"train_mean_token_accuracy"`
	ValidLoss                  float64 `json:"valid_loss"`
	ValidMeanTokenAccuracy     float64 `json:"valid_mean_token_accuracy"`
	FullValidLoss              float64 `json:"full_valid_loss"`
	FullValidMeanTokenAccuracy float64 `json:"full_valid_mean_token_accuracy"`
}

// FineTuningListResponse represents a paginated list response.
type FineTuningListResponse struct {
	Data    []FineTuningJob `json:"data"`
	Object  string          `json:"object"`
	HasMore bool            `json:"has_more"`
}

// FineTuningEventListResponse represents a paginated event list.
type FineTuningEventListResponse struct {
	Data    []FineTuningEvent `json:"data"`
	Object  string            `json:"object"`
	HasMore bool              `json:"has_more"`
}

// FineTuningCheckpointListResponse represents a paginated checkpoint list.
type FineTuningCheckpointListResponse struct {
	Data    []FineTuningCheckpoint `json:"data"`
	Object  string                 `json:"object"`
	HasMore bool                   `json:"has_more"`
}
