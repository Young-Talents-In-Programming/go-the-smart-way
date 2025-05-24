package article

import (
	"time" // Required for timestamps in events
	"errors" // Required for error handling

	"article-manager/internal/events" // Adjusted import path
)

// ArticleAggregate ist die Hauptentität für Artikel.
// Es verwaltet den Zustand eines Artikels und die Geschäftslogik.
type ArticleAggregate struct {
	ID      string
	Title   string
	Content string
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
		// Die Version wird nicht mehr in ApplyEvent für ArticleCreatedEvent gesetzt.
		// Der Aufrufer (HandleCreateArticleCommand oder loadAggregate) ist dafür verantwortlich.
	case *events.ArticleTitleUpdatedEvent:
		a.Title = e.NewTitle
	case *events.ArticleContentUpdatedEvent:
		a.Content = e.NewContent
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
func (a *ArticleAggregate) HandleCreateArticleCommand(id string, title string, content string) error {
	if title == "" {
		return errors.New("Titel darf nicht leer sein")
	}
	if content == "" {
		return errors.New("Inhalt darf nicht leer sein")
	}

	// a.Version ist hier -1. Nach ApplyEvent und dem Setzen wird es 0 sein.
	// Das Event sollte die Version widerspiegeln, die das Aggregat *nach* diesem Event haben wird.
	// Die Variable 'event' war hier deklariert, aber nicht mehr verwendet, nachdem 'createdEvent' eingeführt wurde.
	// Sie wurde entfernt, um den "declared and not used" Fehler zu beheben.
	// Erst ApplyEvent und Version setzen, dann die Version im Event festhalten.
	// Die Logik unten setzt a.Version auf 0 NACH ApplyEvent.
	// Das Event sollte die Version 0 tragen.
	// Korrektur: Event muss nach der Versionsaktualisierung des Aggregats erstellt oder aktualisiert werden.
	// Temporäre Variable für das Event, um die Version später zu setzen.
	// Die erste, fehlerhafte a.recordChange(event) wurde entfernt.
	createdEvent := &events.ArticleCreatedEvent{
		ID:        id,
		Title:     title,
		Content:   content,
		Timestamp: time.Now(),
		// Version wird nach der Aktualisierung von a.Version gesetzt
	}

	if err := a.ApplyEvent(createdEvent); err != nil { // Zustand direkt anwenden und Fehler prüfen
		return err
	}
	a.Version = 0 // Nach dem Erstellen ist die Version des Aggregats 0
	createdEvent.Version = a.Version // Setze die korrekte Version im Event
	a.recordChange(createdEvent) // Zeichne das Event mit der korrekten Version auf
	return nil
}

// HandleUpdateArticleTitleCommand verarbeitet das UpdateArticleTitleCommand.
// Es validiert das Command und erzeugt ein ArticleTitleUpdatedEvent.
func (a *ArticleAggregate) HandleUpdateArticleTitleCommand(newTitle string) error {
	if newTitle == "" {
		return errors.New("Titel darf nicht leer sein")
	}
	if newTitle == a.Title {
		return errors.New("Titel ist identisch, keine Änderung vorgenommen")
	}

	a.IncrementVersion() // Version erhöhen, bevor das Event erstellt wird

	event := &events.ArticleTitleUpdatedEvent{
		ID:        a.ID,
		NewTitle:  newTitle,
		Timestamp: time.Now(),
		Version:   a.Version, // Die neue Version des Aggregats
	}

	// Zustand direkt anwenden
	if err := a.ApplyEvent(event); err != nil {
		a.Version-- // Version zurücksetzen, falls ApplyEvent fehlschlägt
		return err
	}
	a.recordChange(event) // Event erst nach erfolgreicher Anwendung und Versionierung aufzeichnen
	return nil
}

// HandleUpdateArticleContentCommand verarbeitet das UpdateArticleContentCommand.
// Es validiert das Command und erzeugt ein ArticleContentUpdatedEvent.
func (a *ArticleAggregate) HandleUpdateArticleContentCommand(newContent string) error {
	if newContent == "" {
		return errors.New("Inhalt darf nicht leer sein")
	}
	if newContent == a.Content {
		return errors.New("Inhalt ist identisch, keine Änderung vorgenommen")
	}

	a.IncrementVersion() // Version erhöhen, bevor das Event erstellt wird

	event := &events.ArticleContentUpdatedEvent{
		ID:         a.ID,
		NewContent: newContent,
		Timestamp:  time.Now(),
		Version:    a.Version, // Die neue Version des Aggregats
	}
	
	// Zustand direkt anwenden
	if err := a.ApplyEvent(event); err != nil {
		a.Version-- // Version zurücksetzen, falls ApplyEvent fehlschlägt
		return err
	}
	a.recordChange(event) // Event erst nach erfolgreicher Anwendung und Versionierung aufzeichnen
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
	a.IncrementVersion() // Version erhöhen, nachdem das Event erfolgreich angewendet wurde
	deletedEvent.Version = a.Version // Setze die korrekte Version im Event
	a.recordChange(deletedEvent) // Zeichne das Event mit der korrekten Version auf
	return nil
}
