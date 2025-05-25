package handlers

import (
	"article-manager/internal/events"
	"article-manager/internal/readmodels"
	"fmt" // For error wrapping
	"sync"

	"article-manager/internal/article"
	"article-manager/internal/commands"
	"article-manager/internal/eventstore"
	// "article-manager/internal/events" // Not directly used by command handler, but by aggregate
	"log"
)

// EventHandler defines the interface for handling events.
type EventHandler interface {
	HandleEvent(event interface{}) error
}

// ArticleCommandHandler verarbeitet Artikel-bezogene Commands.
type ArticleCommandHandler struct {
	eventStore   eventstore.EventStore
	eventHandler EventHandler // Changed to use the interface
}

// NewArticleCommandHandler erstellt einen neuen ArticleCommandHandler.
func NewArticleCommandHandler(es eventstore.EventStore, eh EventHandler) *ArticleCommandHandler { // Changed to accept the interface
	return &ArticleCommandHandler{
		eventStore:   es,
		eventHandler: eh,
	}
}

// HandleCreateArticle verarbeitet das CreateArticleCommand.
// Es erstellt ein neues ArticleAggregate, führt das Command aus und speichert die resultierenden Events.
func (h *ArticleCommandHandler) HandleCreateArticle(cmd commands.CreateArticleCommand) error {
	// In CQRS ist die ID oft client-seitig generiert (z.B. UUID).
	// Hier wird sie vom Command übernommen.
	// Die erwartete Version für ein neues Aggregat ist -1.
	// NewArticleAggregate initialisiert die Version des Aggregats auf -1.
	expectedVersion := -1
	aggregate := article.NewArticleAggregate(cmd.ID)

	// HandleCreateArticleCommand wendet das Event an und setzt die Version des Aggregats auf 0.
	err := aggregate.HandleCreateArticleCommand(cmd.ID, cmd.Title, cmd.Content, cmd.Price)
	if err != nil {
		return fmt.Errorf("fehler bei der Ausführung von CreateArticleCommand: %w", err)
	}

	changes := aggregate.GetChanges()
	if len(changes) == 0 {
		// Sollte nicht passieren, da HandleCreateArticleCommand immer ein Event erzeugt.
		return fmt.Errorf("CreateArticleCommand erzeugte keine Events für Aggregat %s", cmd.ID)
	}

	err = h.eventStore.SaveEvents(aggregate.ID, changes, expectedVersion)
	if err != nil {
		return fmt.Errorf("fehler beim Speichern der Events für neues Aggregat %s (erwartete Version %d): %w", aggregate.ID, expectedVersion, err)
	}

	// Direkte Weiterleitung der Events an den EventHandler
	// In einem echten System wäre hier ein Event Bus.
	eventsToDispatch := aggregate.GetChanges()
	for _, event := range eventsToDispatch {
		if err := h.eventHandler.HandleEvent(event); err != nil {
			// Fehler loggen, aber der Command war erfolgreich, da die Events gespeichert wurden.
			// Dies unterstreicht die Notwendigkeit eines robusteren Event-Bus/Retry-Mechanismus.
			log.Printf("FEHLER: Read-Model Event-Handler schlug fehl für Event %T Aggregat %s: %v", event, aggregate.ID, err)
		}
	}
	aggregate.ClearChanges() // Änderungen erst nach der Weiterleitung löschen
	return nil
}

// HandleUpdateArticle verarbeitet das UpdateArticleCommand.
// Es lädt das ArticleAggregate, führt das Command aus und speichert die resultierenden Events.
func (h *ArticleCommandHandler) HandleUpdateArticle(cmd any) error {

	switch cmd.(type) {
	case commands.UpdateArticleTitleCommand:
		return h.handleUpdateArticleTitle(cmd.(commands.UpdateArticleTitleCommand))
	case commands.UpdateArticleContentCommand:
		return h.handleUpdateArticleContent(cmd.(commands.UpdateArticleContentCommand))
	case commands.UpdateArticlePriceCommand:
		return h.handleUpdateArticlePrice(cmd.(commands.UpdateArticlePriceCommand))
	}
	return nil
}

func (h *ArticleCommandHandler) handleUpdateArticleTitle(cmd commands.UpdateArticleTitleCommand) error {
	aggregate, err := h.loadAggregate(cmd.ID)
	if err != nil {
		return fmt.Errorf("fehler beim Laden des Aggregats %s für UpdateArticleTitleCommand: %w", cmd.ID, err)
	}

	// Die Version des geladenen Aggregats ist die erwartete Version für den Optimistic Lock.
	expectedVersion := aggregate.Version

	// HandleUpdateArticleCommand wendet das Event an und inkrementiert die Version des Aggregats.
	err = aggregate.HandleUpdateArticleTitleCommand(cmd.Title)
	if err != nil {
		return fmt.Errorf("fehler bei der Ausführung von UpdateArticleTitleCommand für Aggregat %s: %w", cmd.ID, err)
	}

	changes := aggregate.GetChanges()
	if len(changes) == 0 {
		// Keine Änderungen.
		return nil
	}

	err = h.eventStore.SaveEvents(aggregate.ID, changes, expectedVersion)
	if err != nil {
		return fmt.Errorf("fehler beim Speichern der Events für Aggregat %s (erwartete Version %d): %w", aggregate.ID, expectedVersion, err)
	}

	// Direkte Weiterleitung der Events an den EventHandler
	eventsToDispatch := aggregate.GetChanges()
	for _, event := range eventsToDispatch {
		if err := h.eventHandler.HandleEvent(event); err != nil {
			log.Printf("FEHLER: Read-Model Event-Handler schlug fehl für Event %T Aggregat %s: %v", event, aggregate.ID, err)
		}
	}
	aggregate.ClearChanges() // Änderungen erst nach der Weiterleitung löschen
	return nil
}
func (h *ArticleCommandHandler) handleUpdateArticleContent(cmd commands.UpdateArticleContentCommand) error {
	aggregate, err := h.loadAggregate(cmd.ID)
	if err != nil {
		return fmt.Errorf("fehler beim Laden des Aggregats %s für UpdateArticleContentCommand: %w", cmd.ID, err)
	}

	// Die Version des geladenen Aggregats ist die erwartete Version für den Optimistic Lock.
	expectedVersion := aggregate.Version

	// HandleUpdateArticleCommand wendet das Event an und inkrementiert die Version des Aggregats.
	err = aggregate.HandleUpdateArticleContentCommand(cmd.Content)
	if err != nil {
		return fmt.Errorf("fehler bei der Ausführung von UpdateArticleTitleCommand für Aggregat %s: %w", cmd.ID, err)
	}

	changes := aggregate.GetChanges()
	if len(changes) == 0 {
		// Keine Änderungen.
		return nil
	}

	err = h.eventStore.SaveEvents(aggregate.ID, changes, expectedVersion)
	if err != nil {
		return fmt.Errorf("fehler beim Speichern der Events für Aggregat %s (erwartete Version %d): %w", aggregate.ID, expectedVersion, err)
	}

	// Direkte Weiterleitung der Events an den EventHandler
	eventsToDispatch := aggregate.GetChanges()
	for _, event := range eventsToDispatch {
		if err := h.eventHandler.HandleEvent(event); err != nil {
			log.Printf("FEHLER: Read-Model Event-Handler schlug fehl für Event %T Aggregat %s: %v", event, aggregate.ID, err)
		}
	}
	aggregate.ClearChanges() // Änderungen erst nach der Weiterleitung löschen
	return nil
}
func (h *ArticleCommandHandler) handleUpdateArticlePrice(cmd commands.UpdateArticlePriceCommand) error {
	aggregate, err := h.loadAggregate(cmd.ID)
	if err != nil {
		return fmt.Errorf("fehler beim Laden des Aggregats %s für UpdateArticlePriceCommand: %w", cmd.ID, err)
	}

	// Die Version des geladenen Aggregats ist die erwartete Version für den Optimistic Lock.
	expectedVersion := aggregate.Version

	// HandleUpdateArticleCommand wendet das Event an und inkrementiert die Version des Aggregats.
	err = aggregate.HandleUpdateArticlePriceCommand(cmd.Price)
	if err != nil {
		return fmt.Errorf("fehler bei der Ausführung von UpdateArticleTitleCommand für Aggregat %s: %w", cmd.ID, err)
	}

	changes := aggregate.GetChanges()
	if len(changes) == 0 {
		// Keine Änderungen.
		return nil
	}

	err = h.eventStore.SaveEvents(aggregate.ID, changes, expectedVersion)
	if err != nil {
		return fmt.Errorf("fehler beim Speichern der Events für Aggregat %s (erwartete Version %d): %w", aggregate.ID, expectedVersion, err)
	}

	// Direkte Weiterleitung der Events an den EventHandler
	eventsToDispatch := aggregate.GetChanges()
	for _, event := range eventsToDispatch {
		if err := h.eventHandler.HandleEvent(event); err != nil {
			log.Printf("FEHLER: Read-Model Event-Handler schlug fehl für Event %T Aggregat %s: %v", event, aggregate.ID, err)
		}
	}
	aggregate.ClearChanges() // Änderungen erst nach der Weiterleitung löschen
	return nil
}

// HandleDeleteArticle verarbeitet das DeleteArticleCommand.
// Es lädt das ArticleAggregate, führt das Command aus und speichert die resultierenden Events.
func (h *ArticleCommandHandler) HandleDeleteArticle(cmd commands.DeleteArticleCommand) error {
	aggregate, err := h.loadAggregate(cmd.ID)
	if err != nil {
		return fmt.Errorf("fehler beim Laden des Aggregats %s für DeleteArticleCommand: %w", cmd.ID, err)
	}

	// Die Version des geladenen Aggregats ist die erwartete Version für den Optimistic Lock.
	expectedVersion := aggregate.Version

	// HandleDeleteArticleCommand wendet das Event an und inkrementiert die Version des Aggregats.
	err = aggregate.HandleDeleteArticleCommand()
	if err != nil {
		return fmt.Errorf("fehler bei der Ausführung von DeleteArticleCommand für Aggregat %s: %w", cmd.ID, err)
	}

	changes := aggregate.GetChanges()
	if len(changes) == 0 {
		// Sollte nicht passieren, da HandleDeleteArticleCommand immer ein Event erzeugt.
		return fmt.Errorf("DeleteArticleCommand erzeugte keine Events für Aggregat %s", cmd.ID)
	}

	err = h.eventStore.SaveEvents(aggregate.ID, changes, expectedVersion)
	if err != nil {
		return fmt.Errorf("fehler beim Speichern der Events für Aggregat %s (erwartete Version %d): %w", aggregate.ID, expectedVersion, err)
	}

	// Direkte Weiterleitung der Events an den EventHandler
	eventsToDispatch := aggregate.GetChanges()
	for _, event := range eventsToDispatch {
		if err := h.eventHandler.HandleEvent(event); err != nil {
			log.Printf("FEHLER: Read-Model Event-Handler schlug fehl für Event %T Aggregat %s: %v", event, aggregate.ID, err)
		}
	}
	aggregate.ClearChanges() // Änderungen erst nach der Weiterleitung löschen
	return nil
}

// loadAggregate lädt ein Aggregat aus dem EventStore, indem es alle zugehörigen Events abruft
// und auf eine neue Aggregatinstanz anwendet.
// Die Version des Aggregats wird basierend auf den wiedergegebenen Events korrekt gesetzt.
func (h *ArticleCommandHandler) loadAggregate(aggregateID string) (*article.ArticleAggregate, error) {
	historicalEvents, err := h.eventStore.GetEventsForAggregate(aggregateID)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Abrufen der Events für Aggregat %s: %w", aggregateID, err)
	}

	if len(historicalEvents) == 0 {
		// Aggregat existiert nicht, da keine Events vorhanden sind.
		return nil, fmt.Errorf("aggregat %s nicht gefunden: keine Events vorhanden", aggregateID)
	}

	// NewArticleAggregate initialisiert die Version auf -1.
	aggregate := article.NewArticleAggregate(aggregateID)

	for _, event := range historicalEvents {
		err = aggregate.ApplyEvent(event) // ApplyEvent ändert nur den Zustand, nicht die Version.
		if err != nil {
			return nil, fmt.Errorf("fehler beim Anwenden des Events auf Aggregat %s: %w", aggregateID, err)
		}
		aggregate.IncrementVersion() // Version nach jedem angewendeten historischen Event erhöhen.
	}
	// Nach dem Replay aller Events spiegelt aggregate.Version den korrekten aktuellen Stand wider.
	// z.B. 1 Event (create) -> Version 0. 2 Events (create, update) -> Version 1.
	return aggregate, nil
}

// Die Methode saveAggregateEvents wird nicht mehr benötigt, da ihre Logik
// direkt in die Command Handler Methoden integriert wurde.

// -- Event Handler für Read Models --

// Die Imports "log", "article-manager/internal/events", "article-manager/internal/readmodels", "sync"
// sind bereits weiter unten für ArticleEventHandler und ArticleQueryHandler vorhanden oder werden durch Tidy geholt.
// "fmt" ist bereits oben importiert.
// "log" wird für die Fehlerbehandlung bei der Event-Weiterleitung im CommandHandler benötigt.
// Es ist wichtig, dass der log-Import einmal vorhanden ist.

// ArticleEventHandler verarbeitet Events, um die ReadModels zu aktualisieren.
// Dieser Handler wird auch als "Projector" bezeichnet.
type ArticleEventHandler struct {
	mu       sync.RWMutex
	articles map[string]readmodels.ArticleReadModel
}

// NewArticleEventHandler erstellt einen neuen ArticleEventHandler.
func NewArticleEventHandler() *ArticleEventHandler {
	return &ArticleEventHandler{
		articles: make(map[string]readmodels.ArticleReadModel),
	}
}

// HandleEvent verarbeitet ein einzelnes Event und aktualisiert das entsprechende ReadModel.
// In einem realen System würde dieser Handler Events von einem Message Bus oder dem Event Store abonnieren.
func (h *ArticleEventHandler) HandleEvent(event interface{}) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch e := event.(type) {
	case *events.ArticleCreatedEvent:
		// Erstelle ein neues ReadModel für den Artikel.
		// Die Version des Events (e.Version) ist die Version des Aggregats NACH diesem Event.
		newArticle := readmodels.ArticleReadModel{
			ID:      e.ID,
			Title:   e.Title,
			Content: e.Content,
			Price:   e.Price,
			Version: e.Version, // Setze die Version des ReadModels auf die Version des Events
		}
		h.articles[e.ID] = newArticle
		log.Printf("ReadModel für Artikel %s erstellt (Version %d): %+v", e.ID, e.Version, newArticle)

	case *events.ArticleTitleUpdatedEvent:
		// Aktualisiere das bestehende ReadModel.
		currentArticle, ok := h.articles[e.ID]
		if !ok {
			// Falls das ReadModel nicht existiert, könnte es ein Hinweis auf eine verspätete Event-Verarbeitung sein
			// oder der Handler wurde gestartet, nachdem der Artikel bereits erstellt wurde.
			// Für eine robuste Lösung könnte man hier das ReadModel aus dem Event erstellen.
			// Wir loggen eine Warnung und erstellen es neu, falls es fehlt.
			log.Printf("WARNUNG: ArticleUpdatedEvent für nicht existierendes ReadModel ID %s empfangen. ReadModel wird neu erstellt.", e.ID)
			currentArticle = readmodels.ArticleReadModel{ID: e.ID, Version: -1} // Angenommene Version vor dem ersten Update
		}

		// Stelle sicher, dass das Update-Event neuer ist als der aktuelle Stand des Readmodels.
		// e.Version ist die Version des Aggregats NACH diesem Update-Event.
		if e.Version > currentArticle.Version {
			currentArticle.Title = e.Title
			currentArticle.Version = e.Version // Aktualisiere die Version des ReadModels auf die Version des Events
			h.articles[e.ID] = currentArticle
			log.Printf("ReadModel für Artikel %s aktualisiert (Version %d): %+v", e.ID, e.Version, currentArticle)
		} else {
			log.Printf("INFO: Veraltetes oder gleichwertiges ArticleUpdatedEvent für ReadModel ID %s empfangen (EventVersion: %d, ReadModelVersion: %d). Ignoriert.", e.ID, e.Version, currentArticle.Version)
		}

	case *events.ArticleContentUpdatedEvent:
		// Aktualisiere das bestehende ReadModel.
		currentArticle, ok := h.articles[e.ID]
		if !ok {
			// Falls das ReadModel nicht existiert, könnte es ein Hinweis auf eine verspätete Event-Verarbeitung sein
			// oder der Handler wurde gestartet, nachdem der Artikel bereits erstellt wurde.
			// Für eine robuste Lösung könnte man hier das ReadModel aus dem Event erstellen.
			// Wir loggen eine Warnung und erstellen es neu, falls es fehlt.
			log.Printf("WARNUNG: ArticleUpdatedEvent für nicht existierendes ReadModel ID %s empfangen. ReadModel wird neu erstellt.", e.ID)
			currentArticle = readmodels.ArticleReadModel{ID: e.ID, Version: -1} // Angenommene Version vor dem ersten Update
		}

		// Stelle sicher, dass das Update-Event neuer ist als der aktuelle Stand des Readmodels.
		// e.Version ist die Version des Aggregats NACH diesem Update-Event.
		if e.Version > currentArticle.Version {
			currentArticle.Content = e.Content
			currentArticle.Version = e.Version // Aktualisiere die Version des ReadModels auf die Version des Events
			h.articles[e.ID] = currentArticle
			log.Printf("ReadModel für Artikel %s aktualisiert (Version %d): %+v", e.ID, e.Version, currentArticle)
		} else {
			log.Printf("INFO: Veraltetes oder gleichwertiges ArticleUpdatedEvent für ReadModel ID %s empfangen (EventVersion: %d, ReadModelVersion: %d). Ignoriert.", e.ID, e.Version, currentArticle.Version)
		}

	case *events.ArticlePriceUpdatedEvent:
		// Aktualisiere das bestehende ReadModel.
		currentArticle, ok := h.articles[e.ID]
		if !ok {
			// Falls das ReadModel nicht existiert, könnte es ein Hinweis auf eine verspätete Event-Verarbeitung sein
			// oder der Handler wurde gestartet, nachdem der Artikel bereits erstellt wurde.
			// Für eine robuste Lösung könnte man hier das ReadModel aus dem Event erstellen.
			// Wir loggen eine Warnung und erstellen es neu, falls es fehlt.
			log.Printf("WARNUNG: ArticleUpdatedEvent für nicht existierendes ReadModel ID %s empfangen. ReadModel wird neu erstellt.", e.ID)
			currentArticle = readmodels.ArticleReadModel{ID: e.ID, Version: -1} // Angenommene Version vor dem ersten Update
		}

		// Stelle sicher, dass das Update-Event neuer ist als der aktuelle Stand des Readmodels.
		// e.Version ist die Version des Aggregats NACH diesem Update-Event.
		if e.Version > currentArticle.Version {
			currentArticle.Price = e.Price
			currentArticle.Version = e.Version // Aktualisiere die Version des ReadModels auf die Version des Events
			h.articles[e.ID] = currentArticle
			log.Printf("ReadModel für Artikel %s aktualisiert (Version %d): %+v", e.ID, e.Version, currentArticle)
		} else {
			log.Printf("INFO: Veraltetes oder gleichwertiges ArticleUpdatedEvent für ReadModel ID %s empfangen (EventVersion: %d, ReadModelVersion: %d). Ignoriert.", e.ID, e.Version, currentArticle.Version)
		}

	case *events.ArticleDeletedEvent:
		// Entferne das ReadModel.
		// Es ist nicht unbedingt ein Fehler, wenn das ReadModel nicht existiert,
		// z.B. wenn das Delete-Event mehrfach verarbeitet wird.
		if _, ok := h.articles[e.ID]; ok {
			delete(h.articles, e.ID)
			log.Printf("ReadModel für Artikel %s (Version %d) gelöscht.", e.ID, e.Version)
		} else {
			log.Printf("WARNUNG: ArticleDeletedEvent für nicht existierendes ReadModel ID %s empfangen (EventVersion: %d).", e.ID, e.Version)
		}

	default:
		// Unbekanntes Event, ignoriere es oder logge es.
		log.Printf("Unbekanntes Event im ArticleEventHandler empfangen: %T", event)
	}
	return nil
}

// -- Query Handler für Read Models --

// ArticleQueryHandler verarbeitet Abfragen zu Artikel-ReadModels.
type ArticleQueryHandler struct {
	eventHandler *ArticleEventHandler // Referenz zum EventHandler, um auf die ReadModels zuzugreifen
}

// NewArticleQueryHandler erstellt einen neuen ArticleQueryHandler.
func NewArticleQueryHandler(eventHandler *ArticleEventHandler) *ArticleQueryHandler {
	return &ArticleQueryHandler{
		eventHandler: eventHandler,
	}
}

// GetArticleByID ruft ein Artikel-ReadModel anhand seiner ID ab.
func (h *ArticleQueryHandler) GetArticleByID(id string) (readmodels.ArticleReadModel, error) {
	h.eventHandler.mu.RLock() // Lesesperre auf der Mutex des EventHandlers
	defer h.eventHandler.mu.RUnlock()

	existingArticle, ok := h.eventHandler.articles[id]
	if !ok {
		return readmodels.ArticleReadModel{}, fmt.Errorf("artikel mit ID %s nicht gefunden", id)
	}
	return existingArticle, nil
}

// GetAllArticles ruft alle Artikel-ReadModels ab.
func (h *ArticleQueryHandler) GetAllArticles() ([]readmodels.ArticleReadModel, error) {
	h.eventHandler.mu.RLock()
	defer h.eventHandler.mu.RUnlock()

	// Es ist kein Fehler, wenn keine Artikel vorhanden sind; einfach eine leere Liste zurückgeben.
	if len(h.eventHandler.articles) == 0 {
		return []readmodels.ArticleReadModel{}, nil
	}

	articles := make([]readmodels.ArticleReadModel, 0, len(h.eventHandler.articles))
	for _, existingArticle := range h.eventHandler.articles {
		articles = append(articles, existingArticle)
	}
	return articles, nil
}
