package commands

// CreateArticleCommand erstellt einen neuen Artikel.
type CreateArticleCommand struct {
	ID      string
	Title   string
	Content string
	Price   float64
}

// UpdateArticleTitleCommand aktualisiert einen vorhandenen Artikel.
type UpdateArticleTitleCommand struct {
	ID    string
	Title string
}

// UpdateArticleContentCommand aktualisiert einen vorhandenen Artikel.
type UpdateArticleContentCommand struct {
	ID      string
	Content string
}

// UpdateArticlePriceCommand aktualisiert einen vorhandenen Artikel.
type UpdateArticlePriceCommand struct {
	ID    string
	Price float64
}

// DeleteArticleCommand löscht einen Artikel.
type DeleteArticleCommand struct {
	ID string
}

type UpdateCommand interface {
	UpdateArticleTitleCommand | UpdateArticleContentCommand | UpdateArticlePriceCommand
}
