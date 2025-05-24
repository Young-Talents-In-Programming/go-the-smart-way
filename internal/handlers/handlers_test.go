package handlers_test

import (
	"article-manager/internal/commands"
	"article-manager/internal/events"
	// "article-manager/internal/eventstore" // Interface is defined, but we use the mock
	"article-manager/internal/handlers"
	"errors"
	"fmt"
	"strings" // Added import
	"testing"
	"time"

	"github.com/google/uuid"
)

// Helper function to create a new UUID string for tests
func newID() string {
	return uuid.New().String()
}

// --- MockEventStore ---
type MockEventStore struct {
	SaveEventsFunc            func(aggregateID string, events []interface{}, expectedVersion int) error
	GetEventsForAggregateFunc func(aggregateID string) ([]interface{}, error)

	// Call recording fields
	SavedAggID              string
	SavedEvents             []interface{}
	SavedExpectedVersion    int
	GotAggIDForLoad         string
	SaveEventsCalled        bool
	GetEventsForAggCalled bool
}

func (m *MockEventStore) SaveEvents(aggregateID string, evts []interface{}, expectedVersion int) error {
	m.SaveEventsCalled = true
	m.SavedAggID = aggregateID
	m.SavedEvents = evts
	m.SavedExpectedVersion = expectedVersion
	if m.SaveEventsFunc != nil {
		return m.SaveEventsFunc(aggregateID, evts, expectedVersion)
	}
	return nil
}

func (m *MockEventStore) GetEventsForAggregate(aggregateID string) ([]interface{}, error) {
	m.GetEventsForAggCalled = true
	m.GotAggIDForLoad = aggregateID
	if m.GetEventsForAggregateFunc != nil {
		return m.GetEventsForAggregateFunc(aggregateID)
	}
	return []interface{}{}, nil
}

func (m *MockEventStore) Reset() {
	m.SaveEventsCalled = false
	m.GetEventsForAggCalled = false
	m.SavedAggID = ""
	m.SavedEvents = nil
	m.SavedExpectedVersion = -2 // Use a value that's unlikely to be an actual expected version
	m.GotAggIDForLoad = ""
	m.SaveEventsFunc = nil
	m.GetEventsForAggregateFunc = nil
}

// --- MockArticleEventHandler ---
type MockArticleEventHandler struct {
	HandleEventFunc func(event interface{}) error
	HandledEvents   []interface{}
	HandleEventCalled bool
}

func (m *MockArticleEventHandler) HandleEvent(event interface{}) error {
	m.HandleEventCalled = true
	m.HandledEvents = append(m.HandledEvents, event)
	if m.HandleEventFunc != nil {
		return m.HandleEventFunc(event)
	}
	return nil
}

func (m *MockArticleEventHandler) Reset() {
	m.HandleEventCalled = false
	m.HandledEvents = nil
	m.HandleEventFunc = nil
}


// --- Test Suite ---

func TestArticleCommandHandler_HandleCreateArticle(t *testing.T) {
	mockES := &MockEventStore{}
	mockEH := &MockArticleEventHandler{}
	// Note: NewArticleCommandHandler now takes eventHandler as the second argument
	cmdHandler := handlers.NewArticleCommandHandler(mockES, mockEH)

	t.Run("Success", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()

		cmd := commands.CreateArticleCommand{
			ID:      newID(),
			Title:   "New Article",
			Content: "Some content",
		}

		err := cmdHandler.HandleCreateArticle(cmd)
		if err != nil {
			t.Fatalf("HandleCreateArticle failed: %v", err)
		}

		if !mockES.SaveEventsCalled {
			t.Error("expected EventStore.SaveEvents to be called")
		}
		if mockES.SavedAggID != cmd.ID {
			t.Errorf("expected SavedAggID %s, got %s", cmd.ID, mockES.SavedAggID)
		}
		if len(mockES.SavedEvents) != 1 {
			t.Fatalf("expected 1 saved event, got %d", len(mockES.SavedEvents))
		}
		createdEvent, ok := mockES.SavedEvents[0].(*events.ArticleCreatedEvent)
		if !ok {
			t.Fatalf("expected ArticleCreatedEvent, got %T", mockES.SavedEvents[0])
		}
		if createdEvent.ID != cmd.ID || createdEvent.Title != cmd.Title || createdEvent.Content != cmd.Content {
			t.Errorf("event content mismatch. Expected ID %s, Title %s, Content %s. Got ID %s, Title %s, Content %s",
				cmd.ID, cmd.Title, cmd.Content, createdEvent.ID, createdEvent.Title, createdEvent.Content)
		}
		if createdEvent.Version != 0 { // Version of the aggregate after creation
			t.Errorf("expected createdEvent.Version to be 0, got %d", createdEvent.Version)
		}
		if mockES.SavedExpectedVersion != -1 {
			t.Errorf("expected SavedExpectedVersion -1, got %d", mockES.SavedExpectedVersion)
		}

		if !mockEH.HandleEventCalled {
			t.Error("expected EventHandler.HandleEvent to be called")
		}
		if len(mockEH.HandledEvents) != 1 {
			t.Fatalf("expected 1 handled event, got %d", len(mockEH.HandledEvents))
		}
		if mockEH.HandledEvents[0] != createdEvent { // Should be the same event instance
			t.Error("event handler handled a different event instance")
		}
	})

	t.Run("SaveEventsError", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		expectedErr := errors.New("event store save failed")
		mockES.SaveEventsFunc = func(aggregateID string, events []interface{}, expectedVersion int) error {
			return expectedErr
		}

		cmd := commands.CreateArticleCommand{ID: newID(), Title: "Test", Content: "Test"}
		err := cmdHandler.HandleCreateArticle(cmd)

		if err == nil {
			t.Fatal("expected an error from HandleCreateArticle, got nil")
		}
		if !errors.Is(err, expectedErr) && err.Error() != fmt.Errorf("fehler beim Speichern der Events für neues Aggregat %s (erwartete Version -1): %w", cmd.ID, expectedErr).Error() {
            t.Errorf("expected error '%v', got '%v'", expectedErr, err)
        }


		if !mockES.SaveEventsCalled { // SaveEvents should still be called
			t.Error("expected EventStore.SaveEvents to be called")
		}
		if mockEH.HandleEventCalled { // HandleEvent should NOT be called
			t.Error("expected EventHandler.HandleEvent NOT to be called on SaveEvents error")
		}
	})

	t.Run("EventHandlerError", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		eventHandlerErr := errors.New("event handler failed")
		mockEH.HandleEventFunc = func(event interface{}) error {
			return eventHandlerErr
		}

		cmd := commands.CreateArticleCommand{ID: newID(), Title: "Test", Content: "Test"}
		err := cmdHandler.HandleCreateArticle(cmd)

		if err != nil { // Command part is successful, so handler should not return error
			t.Fatalf("HandleCreateArticle returned an error %v, expected nil (event handler error should be logged)", err)
		}

		if !mockES.SaveEventsCalled {
			t.Error("expected EventStore.SaveEvents to be called")
		}
		if !mockEH.HandleEventCalled {
			t.Error("expected EventHandler.HandleEvent to be called")
		}
		// Note: Testing log output is complex. We assume it's logged and the command completes.
	})
}

func TestArticleCommandHandler_HandleUpdateArticleTitle(t *testing.T) {
	mockES := &MockEventStore{}
	mockEH := &MockArticleEventHandler{}
	cmdHandler := handlers.NewArticleCommandHandler(mockES, mockEH)

	articleID := newID()
	initialCreateEvent := &events.ArticleCreatedEvent{
		ID: articleID, Title: "Initial Title", Content: "Initial Content", Version: 0, Timestamp: time.Now(),
	}

	t.Run("Success", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{initialCreateEvent}, nil
		}

		cmd := commands.UpdateArticleTitleCommand{ID: articleID, Title: "New Valid Title"}
		err := cmdHandler.HandleUpdateArticleTitle(cmd)
		if err != nil {
			t.Fatalf("HandleUpdateArticleTitle failed: %v", err)
		}

		if !mockES.SaveEventsCalled {
			t.Error("SaveEvents not called")
		}
		if mockES.SavedExpectedVersion != 0 {
			t.Errorf("Expected version 0, got %d", mockES.SavedExpectedVersion)
		}
		if len(mockES.SavedEvents) != 1 {
			t.Fatal("Expected 1 event")
		}
		evt, ok := mockES.SavedEvents[0].(*events.ArticleTitleUpdatedEvent)
		if !ok {
			t.Fatalf("Expected ArticleTitleUpdatedEvent, got %T", mockES.SavedEvents[0])
		}
		if evt.NewTitle != cmd.Title {
			t.Errorf("Expected title '%s', got '%s'", cmd.Title, evt.NewTitle)
		}
		if evt.Version != 1 { // Version incremented from 0 to 1
			t.Errorf("Expected event version 1, got %d", evt.Version)
		}
		if !mockEH.HandleEventCalled || len(mockEH.HandledEvents) != 1 || mockEH.HandledEvents[0] != evt {
			t.Error("EventHandler not called correctly")
		}
	})

	t.Run("AggregateNotFound", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{}, nil // No events means not found
		}
		cmd := commands.UpdateArticleTitleCommand{ID: articleID, Title: "New Title"}
		err := cmdHandler.HandleUpdateArticleTitle(cmd)
		if err == nil {
			t.Fatal("Expected error for aggregate not found, got nil")
		}
		expectedErrStr := fmt.Sprintf("fehler beim Laden des Aggregats %s für HandleUpdateArticleTitle", articleID)
		if !strings.Contains(err.Error(), expectedErrStr) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedErrStr, err.Error())
		}
	})

	t.Run("OptimisticLockErrorOnSave", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{initialCreateEvent}, nil
		}
		expectedErr := errors.New("optimistic lock error")
		mockES.SaveEventsFunc = func(aggregateID string, evts []interface{}, expectedVersion int) error {
			return expectedErr
		}
		cmd := commands.UpdateArticleTitleCommand{ID: articleID, Title: "New Title"}
		err := cmdHandler.HandleUpdateArticleTitle(cmd)
		if err == nil {
			t.Fatal("Expected optimistic lock error, got nil")
		}
		expectedWrappedError := fmt.Sprintf("fehler beim Speichern der Events für Aggregat %s (erwartete Version 0) in HandleUpdateArticleTitle: %s", articleID, expectedErr.Error())
		if err.Error() != expectedWrappedError {
			t.Errorf("Expected error '%s', got '%s'", expectedWrappedError, err.Error())
		}
	})
    
    t.Run("AggregateReturnsError", func(t *testing.T) { // e.g. title is identical
		mockES.Reset()
		mockEH.Reset()
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{initialCreateEvent}, nil
		}
        // Command uses the same title as initialCreateEvent
		cmd := commands.UpdateArticleTitleCommand{ID: articleID, Title: "Initial Title"} 
		err := cmdHandler.HandleUpdateArticleTitle(cmd)
		if err == nil {
			t.Fatal("Expected error from aggregate (e.g. title identical), got nil")
		}
        expectedErrStr := "Titel ist identisch"
        if !strings.Contains(err.Error(), expectedErrStr) {
            t.Errorf("Expected error to contain '%s', got '%s'", expectedErrStr, err.Error())
        }
		if mockES.SaveEventsCalled {
			t.Error("SaveEvents should not be called if aggregate returns error")
		}
	})
}


func TestArticleCommandHandler_HandleUpdateArticleContent(t *testing.T) {
	mockES := &MockEventStore{}
	mockEH := &MockArticleEventHandler{}
	cmdHandler := handlers.NewArticleCommandHandler(mockES, mockEH)

	articleID := newID()
	initialCreateEvent := &events.ArticleCreatedEvent{
		ID: articleID, Title: "Initial Title", Content: "Initial Content", Version: 0, Timestamp: time.Now(),
	}

	t.Run("Success", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{initialCreateEvent}, nil
		}

		cmd := commands.UpdateArticleContentCommand{ID: articleID, Content: "New Valid Content"}
		err := cmdHandler.HandleUpdateArticleContent(cmd)
		if err != nil {
			t.Fatalf("HandleUpdateArticleContent failed: %v", err)
		}

		if !mockES.SaveEventsCalled {
			t.Error("SaveEvents not called")
		}
		if mockES.SavedExpectedVersion != 0 {
			t.Errorf("Expected version 0, got %d", mockES.SavedExpectedVersion)
		}
		if len(mockES.SavedEvents) != 1 {
			t.Fatal("Expected 1 event")
		}
		evt, ok := mockES.SavedEvents[0].(*events.ArticleContentUpdatedEvent)
		if !ok {
			t.Fatalf("Expected ArticleContentUpdatedEvent, got %T", mockES.SavedEvents[0])
		}
		if evt.NewContent != cmd.Content {
			t.Errorf("Expected content '%s', got '%s'", cmd.Content, evt.NewContent)
		}
		if evt.Version != 1 { // Version incremented from 0 to 1
			t.Errorf("Expected event version 1, got %d", evt.Version)
		}
		if !mockEH.HandleEventCalled || len(mockEH.HandledEvents) != 1 || mockEH.HandledEvents[0] != evt {
			t.Error("EventHandler not called correctly")
		}
	})
    // Add AggregateNotFound, OptimisticLockErrorOnSave, AggregateReturnsError (content identical)
    // similar to HandleUpdateArticleTitle tests
	t.Run("AggregateNotFound", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{}, nil // No events means not found
		}
		cmd := commands.UpdateArticleContentCommand{ID: articleID, Content: "New Content"}
		err := cmdHandler.HandleUpdateArticleContent(cmd)
		if err == nil {
			t.Fatal("Expected error for aggregate not found, got nil")
		}
		expectedErrStr := fmt.Sprintf("fehler beim Laden des Aggregats %s für HandleUpdateArticleContent", articleID)
		if !strings.Contains(err.Error(), expectedErrStr) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedErrStr, err.Error())
		}
	})

	t.Run("OptimisticLockErrorOnSave", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{initialCreateEvent}, nil
		}
		expectedErr := errors.New("optimistic lock error")
		mockES.SaveEventsFunc = func(aggregateID string, evts []interface{}, expectedVersion int) error {
			return expectedErr
		}
		cmd := commands.UpdateArticleContentCommand{ID: articleID, Content: "New Content"}
		err := cmdHandler.HandleUpdateArticleContent(cmd)
		if err == nil {
			t.Fatal("Expected optimistic lock error, got nil")
		}
		expectedWrappedError := fmt.Sprintf("fehler beim Speichern der Events für Aggregat %s (erwartete Version 0) in HandleUpdateArticleContent: %s", articleID, expectedErr.Error())
		if err.Error() != expectedWrappedError {
			t.Errorf("Expected error '%s', got '%s'", expectedWrappedError, err.Error())
		}
	})
    
    t.Run("AggregateReturnsError", func(t *testing.T) { // e.g. content is identical
		mockES.Reset()
		mockEH.Reset()
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{initialCreateEvent}, nil
		}
        // Command uses the same content as initialCreateEvent
		cmd := commands.UpdateArticleContentCommand{ID: articleID, Content: "Initial Content"} 
		err := cmdHandler.HandleUpdateArticleContent(cmd)
		if err == nil {
			t.Fatal("Expected error from aggregate (e.g. content identical), got nil")
		}
        expectedErrStr := "Inhalt ist identisch"
        if !strings.Contains(err.Error(), expectedErrStr) {
            t.Errorf("Expected error to contain '%s', got '%s'", expectedErrStr, err.Error())
        }
		if mockES.SaveEventsCalled {
			t.Error("SaveEvents should not be called if aggregate returns error")
		}
	})
}


func TestArticleCommandHandler_HandleDeleteArticle(t *testing.T) {
	mockES := &MockEventStore{}
	mockEH := &MockArticleEventHandler{}
	cmdHandler := handlers.NewArticleCommandHandler(mockES, mockEH)

	articleID := newID()
	initialCreateEvent := &events.ArticleCreatedEvent{
		ID: articleID, Title: "Initial Title", Content: "Initial Content", Version: 0, Timestamp: time.Now(),
	}

	t.Run("Success", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()

		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			if id != articleID {
				t.Fatalf("GetEventsForAggregate called with wrong ID. Expected %s, got %s", articleID, id)
			}
			return []interface{}{initialCreateEvent}, nil
		}

		cmd := commands.DeleteArticleCommand{ID: articleID}
		err := cmdHandler.HandleDeleteArticle(cmd)
		if err != nil {
			t.Fatalf("HandleDeleteArticle failed: %v", err)
		}

		if !mockES.GetEventsForAggCalled {
			t.Error("expected EventStore.GetEventsForAggregate to be called")
		}
		if !mockES.SaveEventsCalled {
			t.Error("expected EventStore.SaveEvents to be called")
		}
		if mockES.SavedAggID != articleID {
			t.Errorf("expected SavedAggID %s, got %s", articleID, mockES.SavedAggID)
		}
		if len(mockES.SavedEvents) != 1 {
			t.Fatalf("expected 1 saved event, got %d", len(mockES.SavedEvents))
		}
		deletedEvent, ok := mockES.SavedEvents[0].(*events.ArticleDeletedEvent)
		if !ok {
			t.Fatalf("expected ArticleDeletedEvent, got %T", mockES.SavedEvents[0])
		}
		if deletedEvent.ID != cmd.ID {
			t.Errorf("event ID mismatch")
		}
		if deletedEvent.Version != 1 { // Version after delete (0 -> 1)
			t.Errorf("expected deletedEvent.Version to be 1, got %d", deletedEvent.Version)
		}
		if mockES.SavedExpectedVersion != 0 { // Expected version was 0
			t.Errorf("expected SavedExpectedVersion 0, got %d", mockES.SavedExpectedVersion)
		}

		if !mockEH.HandleEventCalled {
			t.Error("expected EventHandler.HandleEvent to be called")
		}
		if len(mockEH.HandledEvents) != 1 {
			t.Fatalf("expected 1 handled event, got %d", len(mockEH.HandledEvents))
		}
		if mockEH.HandledEvents[0] != deletedEvent {
			t.Error("event handler handled a different event instance")
		}
	})
	
	// Similar tests for AggregateNotFound and OptimisticLockError can be added for Delete
	// as they would follow the same pattern as HandleUpdateArticle.
}


// --- Helper functions for creating events ---
func newArticleCreatedEventForTest(id, title, content string, version int) *events.ArticleCreatedEvent {
	return &events.ArticleCreatedEvent{
		ID:        id,
		Title:     title,
		Content:   content,
		Timestamp: time.Now(),
		Version:   version,
	}
}

func newArticleTitleUpdatedEventForTest(id, title string, version int) *events.ArticleTitleUpdatedEvent {
	return &events.ArticleTitleUpdatedEvent{
		ID:        id,
		NewTitle:  title,
		Timestamp: time.Now(),
		Version:   version,
	}
}

func newArticleContentUpdatedEventForTest(id, content string, version int) *events.ArticleContentUpdatedEvent {
	return &events.ArticleContentUpdatedEvent{
		ID:         id,
		NewContent: content,
		Timestamp: time.Now(),
		Version:   version,
	}
}

func newArticleDeletedEventForTest(id string, version int) *events.ArticleDeletedEvent {
	return &events.ArticleDeletedEvent{
		ID:        id,
		Timestamp: time.Now(),
		Version:   version,
	}
}


// --- Test Suite for ArticleEventHandler and ArticleQueryHandler (Integration) ---

func TestArticleEventHandlerAndQueryHandler(t *testing.T) {
	articleID1 := newID()
	initialTitle1 := "Initial Title 1"
	initialContent1 := "Initial Content 1"

	articleID2 := newID()
	initialTitle2 := "Initial Title 2"
	initialContent2 := "Initial Content 2"


	// These handlers will be re-initialized for some test groups for isolation
	var eventHandler *handlers.ArticleEventHandler
	var queryHandler *handlers.ArticleQueryHandler
	
	// Setup for tests that build state sequentially
	setupSequential := func() {
		eventHandler = handlers.NewArticleEventHandler()
		queryHandler = handlers.NewArticleQueryHandler(eventHandler)
	}


	t.Run("EventHandler_ArticleCreated", func(t *testing.T) {
		setupSequential() // Fresh handlers

		createdEvent := newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, 0)
		err := eventHandler.HandleEvent(createdEvent)
		if err != nil {
			t.Fatalf("HandleEvent(created) failed: %v", err)
		}

		rm, err := queryHandler.GetArticleByID(articleID1)
		if err != nil {
			t.Fatalf("GetArticleByID after create failed: %v", err)
		}
		if rm.ID != articleID1 || rm.Title != initialTitle1 || rm.Content != initialContent1 || rm.Version != 0 {
			t.Errorf("unexpected read model state after create: %+v", rm)
		}
	})

	t.Run("EventHandler_ArticleTitleUpdated_Success", func(t *testing.T) {
		setupSequential()
		eventHandler.HandleEvent(newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, 0))

		updatedTitle := "Updated Title Only"
		titleUpdatedEvent := newArticleTitleUpdatedEventForTest(articleID1, updatedTitle, 1)
		err := eventHandler.HandleEvent(titleUpdatedEvent)
		if err != nil {
			t.Fatalf("HandleEvent(ArticleTitleUpdatedEvent) failed: %v", err)
		}

		rm, _ := queryHandler.GetArticleByID(articleID1)
		if rm.Title != updatedTitle || rm.Version != 1 || rm.Content != initialContent1 {
			t.Errorf("unexpected read model state after title update: %+v", rm)
		}
	})

	t.Run("EventHandler_ArticleContentUpdated_Success", func(t *testing.T) {
		setupSequential()
		eventHandler.HandleEvent(newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, 0))

		updatedContent := "Updated Content Only"
		contentUpdatedEvent := newArticleContentUpdatedEventForTest(articleID1, updatedContent, 1)
		err := eventHandler.HandleEvent(contentUpdatedEvent)
		if err != nil {
			t.Fatalf("HandleEvent(ArticleContentUpdatedEvent) failed: %v", err)
		}

		rm, _ := queryHandler.GetArticleByID(articleID1)
		if rm.Content != updatedContent || rm.Version != 1 || rm.Title != initialTitle1 {
			t.Errorf("unexpected read model state after content update: %+v", rm)
		}
	})
    
    t.Run("EventHandler_ArticleTitleUpdated_StaleIgnored", func(t *testing.T) {
		setupSequential()
		eventHandler.HandleEvent(newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, 0))
		eventHandler.HandleEvent(newArticleTitleUpdatedEventForTest(articleID1, "New Title V1", 1)) // Version is 1

		staleEvent := newArticleTitleUpdatedEventForTest(articleID1, "Stale Title V0", 0) // Stale version
		eventHandler.HandleEvent(staleEvent)

        rm, _ := queryHandler.GetArticleByID(articleID1)
        if rm.Title != "New Title V1" || rm.Version != 1 {
            t.Errorf("Stale title update was not ignored. ReadModel: %+v", rm)
        }
    })

    t.Run("EventHandler_ArticleContentUpdated_StaleIgnored", func(t *testing.T) {
		setupSequential()
		eventHandler.HandleEvent(newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, 0))
		eventHandler.HandleEvent(newArticleContentUpdatedEventForTest(articleID1, "New Content V1", 1)) // Version is 1

		staleEvent := newArticleContentUpdatedEventForTest(articleID1, "Stale Content V0", 0) // Stale version
		eventHandler.HandleEvent(staleEvent)

        rm, _ := queryHandler.GetArticleByID(articleID1)
        if rm.Content != "New Content V1" || rm.Version != 1 {
            t.Errorf("Stale content update was not ignored. ReadModel: %+v", rm)
        }
    })

	t.Run("EventHandler_ArticleDeleted", func(t *testing.T) {
		setupSequential()
		// Create initial article
		eventHandler.HandleEvent(newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, 0))
		// Optional: Update title once to ensure delete works on a modified article
		eventHandler.HandleEvent(newArticleTitleUpdatedEventForTest(articleID1, "Before Delete", 1))


		deletedEvent := newArticleDeletedEventForTest(articleID1, 2) // Version after delete is 2
		err := eventHandler.HandleEvent(deletedEvent)
		if err != nil {
			t.Fatalf("HandleEvent(deleted) failed: %v", err)
		}

		_, err = queryHandler.GetArticleByID(articleID1)
		if err == nil {
			t.Errorf("expected error from GetArticleByID after delete, got nil")
		}
		expectedErrStr := fmt.Sprintf("artikel mit ID %s nicht gefunden", articleID1)
		if err.Error() != expectedErrStr {
			t.Errorf("expected error message '%s', got '%s'", expectedErrStr, err.Error())
		}

		allArticles, _ := queryHandler.GetAllArticles()
		if len(allArticles) != 0 {
			t.Errorf("expected 0 articles after delete, got %d", len(allArticles))
		}
	})

	t.Run("EventHandler_UnknownEvent", func(t *testing.T) {
		setupSequential()
		// Create initial article
		createdEvent := newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, 0)
		eventHandler.HandleEvent(createdEvent)

		err := eventHandler.HandleEvent("this is not an event type string")
		if err != nil {
			t.Fatalf("HandleEvent(unknown) failed: %v", err) // Should log and ignore, not fail
		}

		rm, err := queryHandler.GetArticleByID(articleID1)
		if err != nil {
			t.Fatalf("GetArticleByID after unknown event failed: %v", err)
		}
		if rm.Title != initialTitle1 || rm.Content != initialContent1 || rm.Version != 0 {
			t.Errorf("article state changed after unknown event. Got: %+v", rm)
		}
	})

	t.Run("QueryHandler_GetArticleByID_NotFound", func(t *testing.T) {
		setupSequential() // Fresh handlers, no articles

		_, err := queryHandler.GetArticleByID("nonExistentID")
		if err == nil {
			t.Fatal("expected error for GetArticleByID nonExistentID, got nil")
		}
		expectedErrStr := "artikel mit ID nonExistentID nicht gefunden"
		if err.Error() != expectedErrStr {
			t.Errorf("expected error message '%s', got '%s'", expectedErrStr, err.Error())
		}
	})

	t.Run("QueryHandler_GetAllArticles_Multiple", func(t *testing.T) {
		setupSequential()

		event1 := newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, 0)
		event2 := newArticleCreatedEventForTest(articleID2, initialTitle2, initialContent2, 0)
		eventHandler.HandleEvent(event1)
		eventHandler.HandleEvent(event2)

		articles, err := queryHandler.GetAllArticles()
		if err != nil {
			t.Fatalf("GetAllArticles failed: %v", err)
		}
		if len(articles) != 2 {
			t.Fatalf("expected 2 articles, got %d", len(articles))
		}

		// Check for presence of both articles (order is not guaranteed)
		found1, found2 := false, false
		for _, art := range articles {
			if art.ID == articleID1 && art.Title == initialTitle1 {
				found1 = true
			}
			if art.ID == articleID2 && art.Title == initialTitle2 {
				found2 = true
			}
		}
		if !found1 || !found2 {
			t.Errorf("expected both articles to be present. Found1: %t, Found2: %t", found1, found2)
		}
	})

	t.Run("QueryHandler_GetAllArticles_Empty", func(t *testing.T) {
		setupSequential() // Fresh handlers

		articles, err := queryHandler.GetAllArticles()
		if err != nil {
			t.Fatalf("GetAllArticles (empty) failed: %v", err)
		}
		if len(articles) != 0 {
			t.Errorf("expected 0 articles for empty handler, got %d", len(articles))
		}
	})
}
