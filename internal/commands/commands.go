package commands

// CreateArticleCommand erstellt einen neuen Artikel.
type CreateArticleCommand struct {
	ID      string
	Title   string
	Content string
}

// UpdateArticleCommand aktualisiert einen vorhandenen Artikel.
type UpdateArticleCommand struct {
	ID      string
	Title   string
	Content string
}

// DeleteArticleCommand löscht einen Artikel.
type DeleteArticleCommand struct {
	ID string
}
