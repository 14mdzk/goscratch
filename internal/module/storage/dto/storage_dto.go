package dto

// FileResponse represents a file in API responses
type FileResponse struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	URL         string `json:"url,omitempty"`
	ModTime     string `json:"mod_time,omitempty"`
}

// UploadResponse represents the response after a successful upload
type UploadResponse struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// ListFilesResponse represents a list of files
type ListFilesResponse struct {
	Files []FileResponse `json:"files"`
}
