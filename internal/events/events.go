package events

import "time"

// ArticleCreatedEvent wird ausgelöst, wenn ein Artikel erstellt wurde.
type ArticleCreatedEvent struct {
	ID        string
	Title     string
	Content   string
	Price     float64
	Timestamp time.Time
	Version   int // Version des Aggregats nach diesem Event
}

// ArticleUpdatedEvent wird ausgelöst, wenn ein Artikel aktualisiert wurde.
type ArticleTitleUpdatedEvent struct {
	ID        string
	Title     string
	Timestamp time.Time
	Version   int // Version des Aggregats nach diesem Event
}

type ArticleContentUpdatedEvent struct {
	ID        string
	Content   string
	Timestamp time.Time
	Version   int // Version des Aggregats nach diesem Event
}

type ArticlePriceUpdatedEvent struct {
	ID        string
	Price     float64
	Timestamp time.Time
	Version   int // Version des Aggregats nach diesem Event
}

// ArticleDeletedEvent wird ausgelöst, wenn ein Artikel gelöscht wurde.
type ArticleDeletedEvent struct {
	ID        string
	Timestamp time.Time
	Version   int // Version des Aggregats nach diesem Event
}
