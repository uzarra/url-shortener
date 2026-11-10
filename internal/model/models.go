package model

type URL struct {
	ShortID     string
	OriginalURL string
	UserID      string
	DeletedFlag bool
}

type BatchItem struct {
	CorrelationID string
	OriginalURL   string
}

type BatchResult struct {
	CorrelationID string
	ShortURL      string
}

type UserURL struct {
	ShortURL    string
	OriginalURL string
}

type DeleteRequest struct {
	UserID  string
	ShortID string
}
