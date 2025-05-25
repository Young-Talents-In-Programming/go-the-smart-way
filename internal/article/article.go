package article

import (
	"errors" // Required for error handling
	"time"   // Required for timestamps in events

	"article-manager/internal/events" // Adjusted import path
)

// ArticleAggregate ist die Hauptentität für Artikel.
// Es verwaltet den Zustand eines Artikels und die Geschäftslogik.
type ArticleAggregate struct {
	ID      string
	Title   string
	Content string
	Price   float64
	Version int
	// ungespeicherte Änderungen/Events
	changes []interface{}
}

// NewArticleAggregate erstellt eine neue Instanz eines ArticleAggregates.
// Wird verwendet, wenn ein Aggregat aus dem Event-Stream neu erstellt wird.
func NewArticleAggregate(id string) *ArticleAggregate {
	return &ArticleAggregate{
		ID:      id,
		Version: -1, // Version initial auf -1 setzen
		changes: make([]interface{}, 0),
	}
}

// GetChanges gibt die nicht gespeicherten Änderungen zurück.
func (a *ArticleAggregate) GetChanges() []interface{} {
	return a.changes
}

// ClearChanges löscht die nicht gespeicherten Änderungen.
func (a *ArticleAggregate) ClearChanges() {
	a.changes = make([]interface{}, 0)
}

// IncrementVersion erhöht die Version des Aggregats.
func (a *ArticleAggregate) IncrementVersion() {
	a.Version++
}

// ApplyEvent wendet ein Event auf das Aggregat an, um dessen Zustand zu ändern.
// Es wird aufgerufen, wenn das Aggregat aus dem Event Store geladen wird oder wenn ein neues Event erzeugt wird.
func (a *ArticleAggregate) ApplyEvent(event interface{}) error {
	switch e := event.(type) {
	case *events.ArticleCreatedEvent:
		a.ID = e.ID
		a.Title = e.Title
		a.Content = e.Content
		a.Price = e.Price
		// Die Version wird nicht mehr in ApplyEvent für ArticleCreatedEvent gesetzt.
		// Der Aufrufer (HandleCreateArticleCommand oder loadAggregate) ist dafür verantwortlich.
	case *events.ArticleTitleUpdatedEvent:
		a.Title = e.Title
	case *events.ArticleContentUpdatedEvent:
		a.Content = e.Content
	case *events.ArticlePriceUpdatedEvent:
		a.Price = e.Price
	case *events.ArticleDeletedEvent:
		// Hier könnte man einen "gelöscht" Status setzen, falls Soft-Delete gewünscht ist.
		// Für Hard-Delete ist hier eventuell keine Zustandsänderung nötig,
		// aber das Event selbst signalisiert die Löschung.
	default:
		return errors.New("unbekanntes Event-Typ")
	}
	// Version wird beim Laden aus dem Store oder nach erfolgreichem Speichern extern gesetzt/inkrementiert.
	// Hier direkt zu Inkrementieren beim Apply kann zu doppelter Zählung führen.
	return nil
}

// recordChange fügt ein Event zu den ungespeicherten Änderungen hinzu.
func (a *ArticleAggregate) recordChange(event interface{}) {
	a.changes = append(a.changes, event)
}

// HandleCreateArticleCommand verarbeitet das CreateArticleCommand.
// Es validiert das Command und erzeugt ein ArticleCreatedEvent.
func (a *ArticleAggregate) HandleCreateArticleCommand(id string, title string, content string, price float64) error {
	if title == "" {
		return errors.New("Titel darf nicht leer sein")
	}
	if content == "" {
		return errors.New("Inhalt darf nicht leer sein")
	}
	if price < 0.0 {
		return errors.New("Preis darf nicht negativ sein")
	}

	// Korrektur: Event muss nach der Versionsaktualisierung des Aggregats erstellt oder aktualisiert werden.
	// Temporäre Variable für das Event, um die Version später zu setzen.
	createdEvent := &events.ArticleCreatedEvent{
		ID:        id,
		Title:     title,
		Content:   content,
		Price:     price,
		Timestamp: time.Now(),
		// Version wird nach der Aktualisierung von a.Version gesetzt
	}

	if err := a.ApplyEvent(createdEvent); err != nil { // Zustand direkt anwenden und Fehler prüfen
		return err
	}
	a.Version = 0                    // Nach dem Erstellen ist die Version des Aggregats 0
	createdEvent.Version = a.Version // Setze die korrekte Version im Event
	a.recordChange(createdEvent)     // Zeichne das Event mit der korrekten Version auf
	return nil
}

// HandleUpdateArticleTitleCommand verarbeitet das UpdateArticleTitleCommand.
// Es validiert das Command und erzeugt ein ArticleTitleUpdatedEvent.
func (a *ArticleAggregate) HandleUpdateArticleTitleCommand(title string) error {
	if title == "" {
		return errors.New("Titel darf nicht leer sein")
	}
	// Prüfen, ob sich tatsächlich etwas geändert hat (optional, aber gut für die Performance)
	if title == a.Title {
		return nil
	}

	// Temporäre Variable für das Event, um die Version später zu setzen.
	updatedEvent := &events.ArticleTitleUpdatedEvent{
		ID:        a.ID,
		Title:     title,
		Timestamp: time.Now(),
		// Version wird nach der Aktualisierung von a.Version gesetzt
	}

	if err := a.ApplyEvent(updatedEvent); err != nil { // Zustand direkt anwenden und Fehler prüfen
		return err
	}
	a.IncrementVersion()             // Version erhöhen, nachdem das Event erfolgreich angewendet wurde
	updatedEvent.Version = a.Version // Setze die korrekte Version im Event
	a.recordChange(updatedEvent)     // Zeichne das Event mit der korrekten Version auf
	return nil
}

// HandleUpdateArticleContentCommand verarbeitet das UpdateArticleContentCommand.
// Es validiert das Command und erzeugt ein ArticleContentUpdatedEvent.
func (a *ArticleAggregate) HandleUpdateArticleContentCommand(content string) error {
	if content == "" {
		return errors.New("Inhalt darf nicht leer sein")
	}
	// Prüfen, ob sich tatsächlich etwas geändert hat (optional, aber gut für die Performance)
	if content == a.Content {
		return nil
	}

	// Temporäre Variable für das Event, um die Version später zu setzen.
	updatedEvent := &events.ArticleContentUpdatedEvent{
		ID:        a.ID,
		Content:   content,
		Timestamp: time.Now(),
		// Version wird nach der Aktualisierung von a.Version gesetzt
	}

	if err := a.ApplyEvent(updatedEvent); err != nil { // Zustand direkt anwenden und Fehler prüfen
		return err
	}
	a.IncrementVersion()             // Version erhöhen, nachdem das Event erfolgreich angewendet wurde
	updatedEvent.Version = a.Version // Setze die korrekte Version im Event
	a.recordChange(updatedEvent)     // Zeichne das Event mit der korrekten Version auf
	return nil
}

// HandleUpdateArticlePriceCommand verarbeitet das UpdateArticlePriceCommand.
// Es validiert das Command und erzeugt ein ArticlePriceUpdatedEvent.
func (a *ArticleAggregate) HandleUpdateArticlePriceCommand(price float64) error {
	if price < 0.0 {
		return errors.New("Preis darf nicht negativ sein")
	}
	if price == a.Price {
		return nil
	}

	// Temporäre Variable für das Event, um die Version später zu setzen.
	updatedEvent := &events.ArticlePriceUpdatedEvent{
		ID:        a.ID,
		Price:     price,
		Timestamp: time.Now(),
		// Version wird nach der Aktualisierung von a.Version gesetzt
	}

	if err := a.ApplyEvent(updatedEvent); err != nil { // Zustand direkt anwenden und Fehler prüfen
		return err
	}
	a.IncrementVersion()             // Version erhöhen, nachdem das Event erfolgreich angewendet wurde
	updatedEvent.Version = a.Version // Setze die korrekte Version im Event
	a.recordChange(updatedEvent)     // Zeichne das Event mit der korrekten Version auf
	return nil
}

// HandleDeleteArticleCommand verarbeitet das DeleteArticleCommand.
// Es erzeugt ein ArticleDeletedEvent.
func (a *ArticleAggregate) HandleDeleteArticleCommand() error {
	// Hier könnten Prüfungen stattfinden, z.B. ob der Artikel überhaupt existiert (hat eine Version > -1)
	// oder ob er bereits gelöscht ist.

	// Temporäre Variable für das Event, um die Version später zu setzen.
	deletedEvent := &events.ArticleDeletedEvent{
		ID:        a.ID,
		Timestamp: time.Now(),
		// Version wird nach der Aktualisierung von a.Version gesetzt
	}

	if err := a.ApplyEvent(deletedEvent); err != nil { // Zustand direkt anwenden und Fehler prüfen
		return err
	}
	a.IncrementVersion()             // Version erhöhen, nachdem das Event erfolgreich angewendet wurde
	deletedEvent.Version = a.Version // Setze die korrekte Version im Event
	a.recordChange(deletedEvent)     // Zeichne das Event mit der korrekten Version auf
	return nil
}
