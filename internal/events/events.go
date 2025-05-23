package events

import "time"

// ArticleCreatedEvent wird ausgelöst, wenn ein Artikel erstellt wurde.
type ArticleCreatedEvent struct {
	ID        string
	Title     string
	Content   string
	Timestamp time.Time
	Version   int // Version des Aggregats nach diesem Event
}

// ArticleUpdatedEvent wird ausgelöst, wenn ein Artikel aktualisiert wurde.
type ArticleUpdatedEvent struct {
	ID        string
	Title     string
	Content   string
	Timestamp time.Time
	Version   int // Version des Aggregats nach diesem Event
}

// ArticleDeletedEvent wird ausgelöst, wenn ein Artikel gelöscht wurde.
type ArticleDeletedEvent struct {
	ID        string
	Timestamp time.Time
	Version   int // Version des Aggregats nach diesem Event
}
