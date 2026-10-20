package model

type URL struct {
	ShortID     string
	OriginalURL string
}

type BatchItem struct {
	CorrelationID string
	OriginalURL   string
}

type BatchResult struct {
	CorrelationID string
	ShortURL      string
}
