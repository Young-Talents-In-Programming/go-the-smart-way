package handlers

import (
	"fmt"
	"log" // Added for logging
	"sync" // Added for ArticleEventHandler

	"article-manager/internal/article"
	"article-manager/internal/commands"
	"article-manager/internal/eventstore"
	"article-manager/internal/events"     // Added for ArticleEventHandler
	"article-manager/internal/readmodels" // Added for ArticleEventHandler
)

// EventHandler defines the interface for handling events.
// This allows for different implementations, including mocks for testing.
type EventHandler interface {
	HandleEvent(event interface{}) error
}

// ArticleCommandHandler verarbeitet Artikel-bezogene Commands.
type ArticleCommandHandler struct {
	eventStore   eventstore.EventStore
	eventHandler EventHandler // Changed to interface type
}

// NewArticleCommandHandler erstellt einen neuen ArticleCommandHandler.
func NewArticleCommandHandler(es eventstore.EventStore, eh EventHandler) *ArticleCommandHandler {
	return &ArticleCommandHandler{
		eventStore:   es,
		eventHandler: eh, // Uses interface type
	}
}

// HandleCreateArticle verarbeitet das CreateArticleCommand.
// Es erstellt ein neues ArticleAggregate, führt das Command aus und speichert die resultierenden Events.
func (h *ArticleCommandHandler) HandleCreateArticle(cmd commands.CreateArticleCommand) error {
	expectedVersion := -1
	aggregate := article.NewArticleAggregate(cmd.ID)

	err := aggregate.HandleCreateArticleCommand(cmd.ID, cmd.Title, cmd.Content)
	if err != nil {
		return fmt.Errorf("fehler bei der Ausführung von CreateArticleCommand: %w", err)
	}

	changes := aggregate.GetChanges()
	if len(changes) == 0 {
		return fmt.Errorf("CreateArticleCommand erzeugte keine Events für Aggregat %s", cmd.ID)
	}

	err = h.eventStore.SaveEvents(aggregate.ID, changes, expectedVersion)
	if err != nil {
		return fmt.Errorf("fehler beim Speichern der Events für neues Aggregat %s (erwartete Version %d): %w", aggregate.ID, expectedVersion, err)
	}

	eventsToDispatch := aggregate.GetChanges()
	for _, event := range eventsToDispatch {
		if err := h.eventHandler.HandleEvent(event); err != nil {
			log.Printf("FEHLER: Read-Model Event-Handler schlug fehl für Event %T Aggregat %s: %v", event, aggregate.ID, err)
		}
	}
	aggregate.ClearChanges()
	return nil
}

// HandleUpdateArticleTitle verarbeitet das UpdateArticleTitleCommand.
func (h *ArticleCommandHandler) HandleUpdateArticleTitle(cmd commands.UpdateArticleTitleCommand) error {
	aggregate, err := h.loadAggregate(cmd.ID)
	if err != nil {
		return fmt.Errorf("fehler beim Laden des Aggregats %s für HandleUpdateArticleTitle: %w", cmd.ID, err)
	}

	expectedVersion := aggregate.Version

	err = aggregate.HandleUpdateArticleTitleCommand(cmd.Title)
	if err != nil {
		return fmt.Errorf("fehler bei der Ausführung von HandleUpdateArticleTitleCommand für Aggregat %s: %w", cmd.ID, err)
	}

	changes := aggregate.GetChanges()
	if len(changes) == 0 {
		log.Printf("WARNUNG: HandleUpdateArticleTitleCommand für Aggregat %s erzeugte keine Events, obwohl kein Fehler gemeldet wurde.", cmd.ID)
		return nil
	}

	err = h.eventStore.SaveEvents(aggregate.ID, changes, expectedVersion)
	if err != nil {
		return fmt.Errorf("fehler beim Speichern der Events für Aggregat %s (erwartete Version %d) in HandleUpdateArticleTitle: %w", aggregate.ID, expectedVersion, err)
	}

	eventsToDispatch := aggregate.GetChanges()
	for _, event := range eventsToDispatch {
		if err := h.eventHandler.HandleEvent(event); err != nil {
			log.Printf("FEHLER: Read-Model Event-Handler schlug fehl für Event %T Aggregat %s: %v", event, aggregate.ID, err)
		}
	}
	aggregate.ClearChanges()
	return nil
}

// HandleUpdateArticleContent verarbeitet das UpdateArticleContentCommand.
func (h *ArticleCommandHandler) HandleUpdateArticleContent(cmd commands.UpdateArticleContentCommand) error {
	aggregate, err := h.loadAggregate(cmd.ID)
	if err != nil {
		return fmt.Errorf("fehler beim Laden des Aggregats %s für HandleUpdateArticleContent: %w", cmd.ID, err)
	}

	expectedVersion := aggregate.Version

	err = aggregate.HandleUpdateArticleContentCommand(cmd.Content)
	if err != nil {
		return fmt.Errorf("fehler bei der Ausführung von HandleUpdateArticleContentCommand für Aggregat %s: %w", cmd.ID, err)
	}

	changes := aggregate.GetChanges()
	if len(changes) == 0 {
		log.Printf("WARNUNG: HandleUpdateArticleContentCommand für Aggregat %s erzeugte keine Events, obwohl kein Fehler gemeldet wurde.", cmd.ID)
		return nil
	}

	err = h.eventStore.SaveEvents(aggregate.ID, changes, expectedVersion)
	if err != nil {
		return fmt.Errorf("fehler beim Speichern der Events für Aggregat %s (erwartete Version %d) in HandleUpdateArticleContent: %w", aggregate.ID, expectedVersion, err)
	}

	eventsToDispatch := aggregate.GetChanges()
	for _, event := range eventsToDispatch {
		if err := h.eventHandler.HandleEvent(event); err != nil {
			log.Printf("FEHLER: Read-Model Event-Handler schlug fehl für Event %T Aggregat %s: %v", event, aggregate.ID, err)
		}
	}
	aggregate.ClearChanges()
	return nil
}

// HandleDeleteArticle verarbeitet das DeleteArticleCommand.
func (h *ArticleCommandHandler) HandleDeleteArticle(cmd commands.DeleteArticleCommand) error {
	aggregate, err := h.loadAggregate(cmd.ID)
	if err != nil {
		return fmt.Errorf("fehler beim Laden des Aggregats %s für DeleteArticleCommand: %w", cmd.ID, err)
	}

	expectedVersion := aggregate.Version

	err = aggregate.HandleDeleteArticleCommand()
	if err != nil {
		return fmt.Errorf("fehler bei der Ausführung von DeleteArticleCommand für Aggregat %s: %w", cmd.ID, err)
	}

	changes := aggregate.GetChanges()
	if len(changes) == 0 {
		return fmt.Errorf("DeleteArticleCommand erzeugte keine Events für Aggregat %s", cmd.ID)
	}

	err = h.eventStore.SaveEvents(aggregate.ID, changes, expectedVersion)
	if err != nil {
		return fmt.Errorf("fehler beim Speichern der Events für Aggregat %s (erwartete Version %d): %w", aggregate.ID, expectedVersion, err)
	}

	eventsToDispatch := aggregate.GetChanges()
	for _, event := range eventsToDispatch {
		if err := h.eventHandler.HandleEvent(event); err != nil {
			log.Printf("FEHLER: Read-Model Event-Handler schlug fehl für Event %T Aggregat %s: %v", event, aggregate.ID, err)
		}
	}
	aggregate.ClearChanges()
	return nil
}

// loadAggregate lädt ein Aggregat aus dem EventStore.
func (h *ArticleCommandHandler) loadAggregate(aggregateID string) (*article.ArticleAggregate, error) {
	historicalEvents, err := h.eventStore.GetEventsForAggregate(aggregateID)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Abrufen der Events für Aggregat %s: %w", aggregateID, err)
	}

	if len(historicalEvents) == 0 {
		return nil, fmt.Errorf("aggregat %s nicht gefunden: keine Events vorhanden", aggregateID)
	}

	aggregate := article.NewArticleAggregate(aggregateID)
	for _, event := range historicalEvents {
		err = aggregate.ApplyEvent(event)
		if err != nil {
			return nil, fmt.Errorf("fehler beim Anwenden des Events auf Aggregat %s: %w", aggregateID, err)
		}
		aggregate.IncrementVersion()
	}
	return aggregate, nil
}

// -- Event Handler für Read Models --

// ArticleEventHandler verarbeitet Events, um die ReadModels zu aktualisieren.
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
func (h *ArticleEventHandler) HandleEvent(event interface{}) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch e := event.(type) {
	case *events.ArticleCreatedEvent:
		newArticle := readmodels.ArticleReadModel{
			ID:      e.ID,
			Title:   e.Title,
			Content: e.Content,
			Version: e.Version,
		}
		h.articles[e.ID] = newArticle
		log.Printf("ReadModel für Artikel %s erstellt (Version %d): %+v", e.ID, e.Version, newArticle)

	case *events.ArticleTitleUpdatedEvent:
		currentArticle, ok := h.articles[e.ID]
		if !ok {
			log.Printf("WARNUNG: ArticleTitleUpdatedEvent für nicht existierendes ReadModel ID %s empfangen. Event ignoriert.", e.ID)
			return nil
		}
		if e.Version > currentArticle.Version {
			currentArticle.Title = e.NewTitle
			currentArticle.Version = e.Version
			h.articles[e.ID] = currentArticle
			log.Printf("ReadModel Titel für Artikel %s aktualisiert (Version %d): %+v", e.ID, e.Version, currentArticle)
		} else {
			log.Printf("INFO: Veraltetes oder gleichwertiges ArticleTitleUpdatedEvent für ReadModel ID %s empfangen (EventVersion: %d, ReadModelVersion: %d). Ignoriert.", e.ID, e.Version, currentArticle.Version)
		}

	case *events.ArticleContentUpdatedEvent:
		currentArticle, ok := h.articles[e.ID]
		if !ok {
			log.Printf("WARNUNG: ArticleContentUpdatedEvent für nicht existierendes ReadModel ID %s empfangen. Event ignoriert.", e.ID)
			return nil
		}
		if e.Version > currentArticle.Version {
			currentArticle.Content = e.NewContent
			currentArticle.Version = e.Version
			h.articles[e.ID] = currentArticle
			log.Printf("ReadModel Inhalt für Artikel %s aktualisiert (Version %d): %+v", e.ID, e.Version, currentArticle)
		} else {
			log.Printf("INFO: Veraltetes oder gleichwertiges ArticleContentUpdatedEvent für ReadModel ID %s empfangen (EventVersion: %d, ReadModelVersion: %d). Ignoriert.", e.ID, e.Version, currentArticle.Version)
		}

	case *events.ArticleDeletedEvent:
		if _, ok := h.articles[e.ID]; ok {
			delete(h.articles, e.ID)
			log.Printf("ReadModel für Artikel %s (Version %d) gelöscht.", e.ID, e.Version)
		} else {
			log.Printf("WARNUNG: ArticleDeletedEvent für nicht existierendes ReadModel ID %s empfangen (EventVersion: %d).", e.ID, e.Version)
		}
	
	default:
		log.Printf("Unbekanntes Event im ArticleEventHandler empfangen: %T", event)
	}
	return nil
}

// -- Query Handler für Read Models --

// ArticleQueryHandler verarbeitet Abfragen zu Artikel-ReadModels.
type ArticleQueryHandler struct {
	eventHandler *ArticleEventHandler
}

// NewArticleQueryHandler erstellt einen neuen ArticleQueryHandler.
func NewArticleQueryHandler(eventHandler *ArticleEventHandler) *ArticleQueryHandler {
	return &ArticleQueryHandler{
		eventHandler: eventHandler,
	}
}

// GetArticleByID ruft ein Artikel-ReadModel anhand seiner ID ab.
func (h *ArticleQueryHandler) GetArticleByID(id string) (readmodels.ArticleReadModel, error) {
	h.eventHandler.mu.RLock()
	defer h.eventHandler.mu.RUnlock()

	article, ok := h.eventHandler.articles[id]
	if !ok {
		return readmodels.ArticleReadModel{}, fmt.Errorf("artikel mit ID %s nicht gefunden", id)
	}
	return article, nil
}

// GetAllArticles ruft alle Artikel-ReadModels ab.
func (h *ArticleQueryHandler) GetAllArticles() ([]readmodels.ArticleReadModel, error) {
	h.eventHandler.mu.RLock()
	defer h.eventHandler.mu.RUnlock()

	if len(h.eventHandler.articles) == 0 {
		return []readmodels.ArticleReadModel{}, nil
	}

	articles := make([]readmodels.ArticleReadModel, 0, len(h.eventHandler.articles))
	for _, article := range h.eventHandler.articles {
		articles = append(articles, article)
	}
	return articles, nil
}
