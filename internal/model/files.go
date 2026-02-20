package model

// FileObject represents an OpenAI file object.
type FileObject struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	Bytes     int64  `json:"bytes"`
	CreatedAt int64  `json:"created_at"`
	Filename  string `json:"filename"`
	Purpose   string `json:"purpose"`
	Status    string `json:"status,omitempty"`
}

// FileListResponse represents the response for listing files.
type FileListResponse struct {
	Data   []FileObject `json:"data"`
	Object string       `json:"object"`
}

// FileDeleteResponse represents the response for deleting a file.
type FileDeleteResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}
