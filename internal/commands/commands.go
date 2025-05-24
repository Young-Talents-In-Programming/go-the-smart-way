package commands

// CreateArticleCommand erstellt einen neuen Artikel.
type CreateArticleCommand struct {
	ID      string
	Title   string
	Content string
}

// UpdateArticleTitleCommand aktualisiert den Titel eines Artikels.
type UpdateArticleTitleCommand struct {
	ID    string
	Title string
}

// UpdateArticleContentCommand aktualisiert den Inhalt eines Artikels.
type UpdateArticleContentCommand struct {
	ID      string
	Content string
}

// DeleteArticleCommand löscht einen Artikel.
type DeleteArticleCommand struct {
	ID string
}
