package models

// Album is both the DB record and the API response shape.
type Album struct {
	AlbumID     string `json:"album_id"     dynamodbav:"album_id"`
	Title       string `json:"title"        dynamodbav:"title"`
	Description string `json:"description"  dynamodbav:"description"`
	Owner       string `json:"owner"        dynamodbav:"owner"`
}

// Photo is both the DB record and the GET /photos/:id response shape.
type Photo struct {
	PhotoID string `json:"photo_id"         dynamodbav:"photo_id"`
	AlbumID string `json:"album_id"         dynamodbav:"album_id"`
	Seq     int    `json:"seq"              dynamodbav:"seq"`
	Status  string `json:"status"           dynamodbav:"status"`
	URL     string `json:"url,omitempty"    dynamodbav:"url,omitempty"`
}

// PhotoAccepted is the 202 response body.
type PhotoAccepted struct {
	PhotoID string `json:"photo_id"`
	Seq     int    `json:"seq"`
	Status  string `json:"status"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}