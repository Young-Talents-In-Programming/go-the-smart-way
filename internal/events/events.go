package events

import "time"

// ArticleCreatedEvent wird ausgelöst, wenn ein Artikel erstellt wurde.
type ArticleCreatedEvent struct {
	ID        string
	Title     string
	Content   string
	Timestamp time.Time
	Version   int
}

// ArticleTitleUpdatedEvent wird ausgelöst, wenn der Titel eines Artikels aktualisiert wurde.
type ArticleTitleUpdatedEvent struct {
	ID        string
	NewTitle  string
	Timestamp time.Time
	Version   int
}

// ArticleContentUpdatedEvent wird ausgelöst, wenn der Inhalt eines Artikels aktualisiert wurde.
type ArticleContentUpdatedEvent struct {
	ID         string
	NewContent string
	Timestamp  time.Time
	Version    int
}

// ArticleDeletedEvent wird ausgelöst, wenn ein Artikel gelöscht wurde.
type ArticleDeletedEvent struct {
	ID        string
	Timestamp time.Time
	Version   int
}
