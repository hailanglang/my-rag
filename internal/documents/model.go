package documents

// Document is a row in the documents table.
type Document struct {
	ID           string  `json:"id"`
	Filename     string  `json:"filename"`
	Mime         string  `json:"mime"`
	Size         int64   `json:"size"`
	StoragePath  string  `json:"-"`
	Status       string  `json:"status"`
	ErrorMessage *string `json:"error_message,omitempty"`
	CreatedAt    int64   `json:"created_at"`
	UpdatedAt    int64   `json:"updated_at"`
}
