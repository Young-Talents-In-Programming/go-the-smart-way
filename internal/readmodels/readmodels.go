package readmodels

// ArticleReadModel ist eine denormalisierte Repräsentation eines Artikels für Lesezugriffe.
// Es wird durch Events aus dem System aktualisiert.
type ArticleReadModel struct {
	// ID ist die eindeutige Kennung des Artikels.
	ID string
	// Title ist der Titel des Artikels.
	Title string
	// Content ist der Inhalt des Artikels.
	Content string
	// Version gibt die Version des Aggregats an, auf der dieses ReadModel basiert.
	// Dies hilft bei der Handhabung von out-of-order Events oder bei der Implementierung
	// von Optimistic Locking auf der Lese-Seite, falls erforderlich.
	Version int
}
