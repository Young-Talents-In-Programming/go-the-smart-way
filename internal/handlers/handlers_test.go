package handlers_test

import (
	"article-manager/internal/commands"
	"article-manager/internal/events"
	// "article-manager/internal/eventstore" // Interface is defined, but we use the mock
	"article-manager/internal/handlers"
	"errors"
	"fmt"
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
			Price:   9.99,
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
		if createdEvent.ID != cmd.ID || createdEvent.Title != cmd.Title || createdEvent.Content != cmd.Content || createdEvent.Price != cmd.Price {
			t.Errorf("event content mismatch. Expected ID %s, Title %s, Content %s, Price %f. Got ID %s, Title %s, Content %s, Price %f",
				cmd.ID, cmd.Title, cmd.Content, cmd.Price, createdEvent.ID, createdEvent.Title, createdEvent.Content, createdEvent.Price)
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

		cmd := commands.CreateArticleCommand{ID: newID(), Title: "Test", Content: "Test", Price: 5.55}
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

		cmd := commands.CreateArticleCommand{ID: newID(), Title: "Test", Content: "Test", Price: 6.66}
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


func TestArticleCommandHandler_HandleUpdateArticle(t *testing.T) {
	mockES := &MockEventStore{}
	mockEH := &MockArticleEventHandler{}
	cmdHandler := handlers.NewArticleCommandHandler(mockES, mockEH)
	
	articleID := newID()
	initialPrice := 10.00
	initialCreateEvent := &events.ArticleCreatedEvent{
		ID: articleID, Title: "Initial Title", Content: "Initial Content", Price: initialPrice, Version: 0, Timestamp: time.Now(),
	}

	// This test will be adapted to test UpdateArticleTitleCommand specifically
	t.Run("Success_UpdateTitle", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()

		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			if id != articleID {
				t.Fatalf("GetEventsForAggregate called with wrong ID. Expected %s, got %s", articleID, id)
			}
			return []interface{}{initialCreateEvent}, nil
		}

		updatedTitle := "Updated Title"
		// Send UpdateArticleTitleCommand
		titleCmd := commands.UpdateArticleTitleCommand{
			ID:    articleID,
			Title: updatedTitle,
		}

		// The main HandleUpdateArticle will receive this specific command
		err := cmdHandler.HandleUpdateArticle(titleCmd)
		if err != nil {
			t.Fatalf("HandleUpdateArticle(UpdateArticleTitleCommand) failed: %v", err)
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
		// Expect ArticleTitleUpdatedEvent
		titleUpdatedEvent, ok := mockES.SavedEvents[0].(*events.ArticleTitleUpdatedEvent)
		if !ok {
			t.Fatalf("expected ArticleTitleUpdatedEvent, got %T", mockES.SavedEvents[0])
		}
		if titleUpdatedEvent.ID != titleCmd.ID || titleUpdatedEvent.Title != titleCmd.Title {
			t.Errorf("event content mismatch. Expected ID %s, Title %s. Got ID %s, Title %s",
				titleCmd.ID, titleCmd.Title, titleUpdatedEvent.ID, titleUpdatedEvent.Title)
		}
		if titleUpdatedEvent.Version != 1 { // Version after update (0 -> 1)
			t.Errorf("expected titleUpdatedEvent.Version to be 1, got %d", titleUpdatedEvent.Version)
		}
		if mockES.SavedExpectedVersion != 0 { // Expected version was 0 (version of loaded aggregate)
			t.Errorf("expected SavedExpectedVersion 0, got %d", mockES.SavedExpectedVersion)
		}

		if !mockEH.HandleEventCalled {
			t.Error("expected EventHandler.HandleEvent to be called")
		}
		if len(mockEH.HandledEvents) != 1 {
			t.Fatalf("expected 1 handled event, got %d", len(mockEH.HandledEvents))
		}
		if mockEH.HandledEvents[0] != titleUpdatedEvent {
			t.Error("event handler handled a different event instance")
		}
	})

	// This subtest needs to be adapted for a specific command or removed if generic updates are not supported.
	// For now, let's assume it tests trying to update a non-existent aggregate with a Title command.
	t.Run("AggregateNotFound_UpdateTitle", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()

		// notFoundErrText := fmt.Sprintf("aggregat %s nicht gefunden: keine Events vorhanden", articleID) // This variable is unused
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{}, nil // Simulate not found by returning no events
		}

		titleCmd := commands.UpdateArticleTitleCommand{ID: articleID, Title: "Update Title For NonExistent"}
		err := cmdHandler.HandleUpdateArticle(titleCmd) // Using the main dispatcher

		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		// Check if the error message matches the expected one from loadAggregate for UpdateArticleTitleCommand
		// The error message from loadAggregateAndHandle does not currently include the command type.
		// It's "fehler beim Laden des Aggregats %s: %w"
		// The specific error from aggregate.Rehydrate is "aggregat %s nicht gefunden: keine Events vorhanden"
		expectedLoadErr := fmt.Sprintf("aggregat %s nicht gefunden: keine Events vorhanden", articleID) // articleID is available in this scope
		expectedWrappedError := fmt.Sprintf("fehler beim Laden des Aggregats %s für UpdateArticleTitleCommand: %s", articleID, expectedLoadErr)
		if err.Error() != expectedWrappedError {
			t.Errorf("expected error message '%s', got '%s'", expectedWrappedError, err.Error())
		}

		if !mockES.GetEventsForAggCalled {
			t.Error("expected GetEventsForAggregate to be called")
		}
		if mockES.SaveEventsCalled {
			t.Error("expected SaveEvents NOT to be called")
		}
		if mockEH.HandleEventCalled {
			t.Error("expected HandleEvent NOT to be called")
		}
	})

    // This subtest also needs to be adapted for a specific command.
	// Let's test optimistic lock for UpdateArticleTitleCommand.
	t.Run("OptimisticLockError_UpdateTitle", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{initialCreateEvent}, nil // Aggregate is at version 0
		}
		optimisticLockErr := errors.New("optimistic lock error: version mismatch")
		mockES.SaveEventsFunc = func(aggregateID string, evts []interface{}, expectedVersion int) error {
			// This mock simulates that SaveEvents itself returns the optimistic lock error.
			// In a real scenario, this error originates from the eventstore's SaveEvents method.
			return optimisticLockErr 
		}

		titleCmd := commands.UpdateArticleTitleCommand{ID: articleID, Title: "Update Title With Lock Error"}
		err := cmdHandler.HandleUpdateArticle(titleCmd)

		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		// The error from saveAndPublishEvents is "fehler beim Speichern der Events für Aggregat %s (erwartete Version %d): %w"
        expectedWrappedError := fmt.Sprintf("fehler beim Speichern der Events für Aggregat %s (erwartete Version 0): %s", articleID, optimisticLockErr.Error())
		if err.Error() != expectedWrappedError {
			t.Errorf("expected error message '%s', got '%s'", expectedWrappedError, err.Error())
		}

		if !mockES.GetEventsForAggCalled {
			t.Error("expected GetEventsForAggregate to be called")
		}
		if !mockES.SaveEventsCalled {
			t.Error("expected SaveEvents to be called")
		}
		if mockEH.HandleEventCalled {
			t.Error("expected HandleEvent NOT to be called")
		}
	})
}

func TestArticleCommandHandler_HandleDeleteArticle(t *testing.T) {
	mockES := &MockEventStore{}
	mockEH := &MockArticleEventHandler{}
	cmdHandler := handlers.NewArticleCommandHandler(mockES, mockEH)

	articleID := newID()
	initialDeletePrice := 3.33
	initialCreateEventForDelete := &events.ArticleCreatedEvent{
		ID: articleID, Title: "Initial Title", Content: "Initial Content", Price: initialDeletePrice, Version: 0, Timestamp: time.Now(),
	}

	t.Run("Success", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()

		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			if id != articleID {
				t.Fatalf("GetEventsForAggregate called with wrong ID. Expected %s, got %s", articleID, id)
			}
			return []interface{}{initialCreateEventForDelete}, nil
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
func newArticleCreatedEventForTest(id, title, content string, price float64, version int) *events.ArticleCreatedEvent {
	return &events.ArticleCreatedEvent{
		ID:        id,
		Title:     title,
		Content:   content,
		Price:     price,
		Timestamp: time.Now(),
		Version:   version,
	}
}

// Renamed from newArticleUpdatedEventForTest
func newArticleTitleUpdatedEventForTest(id, title string, version int) *events.ArticleTitleUpdatedEvent {
	return &events.ArticleTitleUpdatedEvent{
		ID:        id,
		Title:     title,
		Timestamp: time.Now(),
		Version:   version,
	}
}

// Added for completeness, though not strictly required by price tests yet
func newArticleContentUpdatedEventForTest(id, content string, version int) *events.ArticleContentUpdatedEvent {
	return &events.ArticleContentUpdatedEvent{
		ID:        id,
		Content:   content,
		Timestamp: time.Now(),
		Version:   version,
	}
}

func newArticlePriceUpdatedEventForTest(id string, price float64, version int) *events.ArticlePriceUpdatedEvent {
	return &events.ArticlePriceUpdatedEvent{
		ID:        id,
		Price:     price,
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
	initialPrice1 := 11.11

	articleID2 := newID()
	initialTitle2 := "Initial Title 2"
	initialContent2 := "Initial Content 2"
	initialPrice2 := 22.22


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

		createdEvent := newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, initialPrice1, 0)
		err := eventHandler.HandleEvent(createdEvent)
		if err != nil {
			t.Fatalf("HandleEvent(created) failed: %v", err)
		}

		rm, err := queryHandler.GetArticleByID(articleID1)
		if err != nil {
			t.Fatalf("GetArticleByID after create failed: %v", err)
		}
		if rm.ID != articleID1 {
			t.Errorf("expected ID %s, got %s", articleID1, rm.ID)
		}
		if rm.Title != initialTitle1 {
			t.Errorf("expected Title '%s', got '%s'", initialTitle1, rm.Title)
		}
		if rm.Content != initialContent1 {
			t.Errorf("expected Content '%s', got '%s'", initialContent1, rm.Content)
		}
		if rm.Price != initialPrice1 {
			t.Errorf("expected Price %f, got %f", initialPrice1, rm.Price)
		}
		if rm.Version != 0 {
			t.Errorf("expected Version 0, got %d", rm.Version)
		}
	})

	t.Run("EventHandler_ArticleTitleUpdated_Success", func(t *testing.T) {
		setupSequential() // Start fresh for this test sequence
		// Create initial article
		createdEvent := newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, initialPrice1, 0)
		eventHandler.HandleEvent(createdEvent)

		updatedTitle := "Updated Title 1"
		// Use newArticleTitleUpdatedEventForTest
		updatedTitleEvent := newArticleTitleUpdatedEventForTest(articleID1, updatedTitle, 1)
		
		err := eventHandler.HandleEvent(updatedTitleEvent)
		if err != nil {
			t.Fatalf("HandleEvent(title updated) failed: %v", err)
		}

		rm, err := queryHandler.GetArticleByID(articleID1)
		if err != nil {
			t.Fatalf("GetArticleByID after title update failed: %v", err)
		}
		if rm.Title != updatedTitle {
			t.Errorf("expected updated Title '%s', got '%s'", updatedTitle, rm.Title)
		}
		if rm.Content != initialContent1 { // Content should remain the same
			t.Errorf("expected Content '%s', got '%s'", initialContent1, rm.Content)
		}
		if rm.Price != initialPrice1 { // Price should remain the same
			t.Errorf("expected Price %f, got %f", initialPrice1, rm.Price)
		}
		if rm.Version != 1 { // Version should be updated
			t.Errorf("expected Version 1 after title update, got %d", rm.Version)
		}
	})
    
	t.Run("EventHandler_ArticlePriceUpdated_Success", func(t *testing.T) {
		setupSequential()
		createdEvent := newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, initialPrice1, 0)
		eventHandler.HandleEvent(createdEvent)

		updatedPrice := 15.99
		updatedPriceEvent := newArticlePriceUpdatedEventForTest(articleID1, updatedPrice, 1)

		err := eventHandler.HandleEvent(updatedPriceEvent)
		if err != nil {
			t.Fatalf("HandleEvent(price updated) failed: %v", err)
		}
		rm, err := queryHandler.GetArticleByID(articleID1)
		if err != nil {
			t.Fatalf("GetArticleByID after price update failed: %v", err)
		}
		if rm.Title != initialTitle1 { // Title should remain the same
			t.Errorf("expected Title '%s', got '%s'", initialTitle1, rm.Title)
		}
		if rm.Price != updatedPrice {
			t.Errorf("expected Price %f, got %f", updatedPrice, rm.Price)
		}
		if rm.Version != 1 {
			t.Errorf("expected Version 1 after price update, got %d", rm.Version)
		}
	})

    t.Run("EventHandler_ArticleUpdates_StaleIgnored", func(t *testing.T) {
		setupSequential()
		// Create initial article
		createdEvent := newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, initialPrice1, 0) // Version 0
		eventHandler.HandleEvent(createdEvent)
		
		// First update (Title)
		firstUpdateTitle := "First Update Title"
		firstTitleUpdateEvent := newArticleTitleUpdatedEventForTest(articleID1, firstUpdateTitle, 1) // Version 1
		eventHandler.HandleEvent(firstTitleUpdateEvent)

		// Second update (Price)
		currentPrice := 50.55
		secondPriceUpdateEvent := newArticlePriceUpdatedEventForTest(articleID1, currentPrice, 2) // Version 2
		eventHandler.HandleEvent(secondPriceUpdateEvent)

        // Try to apply stale Title update (version 0)
		staleTitleEvent := newArticleTitleUpdatedEventForTest(articleID1, "Stale Title", 0) 
		err := eventHandler.HandleEvent(staleTitleEvent)
		if err != nil {
			t.Fatalf("HandleEvent(stale title update) should not fail: %v", err) 
		}
        rm, _ := queryHandler.GetArticleByID(articleID1)
        if rm.Title != firstUpdateTitle { 
            t.Errorf("expected Title to be '%s' (from first update), got '%s'", firstUpdateTitle, rm.Title)
        }
        if rm.Version != 2 { 
            t.Errorf("expected Version to be 2, got %d", rm.Version)
        }
		if rm.Price != currentPrice {
			t.Errorf("expected Price to be %f, got %f", currentPrice, rm.Price)
		}

		// Try to apply stale Price update (version 1)
		stalePriceEvent := newArticlePriceUpdatedEventForTest(articleID1, 9.99, 1)
		err = eventHandler.HandleEvent(stalePriceEvent)
		if err != nil {
			t.Fatalf("HandleEvent(stale price update) should not fail: %v", err)
		}
		rm, _ = queryHandler.GetArticleByID(articleID1)
        if rm.Price != currentPrice { 
            t.Errorf("expected Price to be %f (from second update), got %f", currentPrice, rm.Price)
        }
        if rm.Version != 2 { 
            t.Errorf("expected Version to be 2, got %d", rm.Version)
        }

		// Try to apply Price update with current version (should also be ignored)
		ignoredPriceEventSameVersion := newArticlePriceUpdatedEventForTest(articleID1, 100.00, 2)
		err = eventHandler.HandleEvent(ignoredPriceEventSameVersion)
		if err != nil {
			t.Fatalf("HandleEvent(ignored price update with same version) should not fail: %v", err)
		}
		rm, _ = queryHandler.GetArticleByID(articleID1)
        if rm.Price != currentPrice { 
            t.Errorf("expected Price to be %f, got %f, after ignored same version update", currentPrice, rm.Price)
        }
        if rm.Version != 2 { 
            t.Errorf("expected Version to be 2, got %d, after ignored same version update", rm.Version)
        }
    })

	t.Run("EventHandler_ArticleDeleted", func(t *testing.T) {
		setupSequential()
		// Create initial article
		createdEvent := newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, initialPrice1, 0)
		eventHandler.HandleEvent(createdEvent)
		// Optional: Update its title once
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
		createdEvent := newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, initialPrice1, 0)
		eventHandler.HandleEvent(createdEvent)

		err := eventHandler.HandleEvent("this is not an event type string")
		if err != nil {
			t.Fatalf("HandleEvent(unknown) failed: %v", err) // Should log and ignore, not fail
		}

		rm, err := queryHandler.GetArticleByID(articleID1)
		if err != nil {
			t.Fatalf("GetArticleByID after unknown event failed: %v", err)
		}
		if rm.Title != initialTitle1 || rm.Content != initialContent1 || rm.Price != initialPrice1 || rm.Version != 0 {
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

		event1 := newArticleCreatedEventForTest(articleID1, initialTitle1, initialContent1, initialPrice1, 0)
		event2 := newArticleCreatedEventForTest(articleID2, initialTitle2, initialContent2, initialPrice2, 0)
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
		var art1Check, art2Check bool
		for _, art := range articles {
			if art.ID == articleID1 {
				art1Check = true
				if art.Title == initialTitle1 && art.Price == initialPrice1 {
					found1 = true
				} else {
					t.Errorf("Article 1 data mismatch. Got Title: %s, Price: %f. Expected Title: %s, Price: %f",
						art.Title, art.Price, initialTitle1, initialPrice1)
				}
			}
			if art.ID == articleID2 {
				art2Check = true
				if art.Title == initialTitle2 && art.Price == initialPrice2 {
					found2 = true
				} else {
					t.Errorf("Article 2 data mismatch. Got Title: %s, Price: %f. Expected Title: %s, Price: %f",
						art.Title, art.Price, initialTitle2, initialPrice2)
				}
			}
		}
		if !art1Check {
			t.Errorf("Article 1 (ID: %s) not found in GetAllArticles result", articleID1)
		}
		if !art2Check {
			t.Errorf("Article 2 (ID: %s) not found in GetAllArticles result", articleID2)
		}
		if !found1 || !found2 { // Redundant if artXCheck fails, but good for overall status
			t.Errorf("expected both articles to be present with correct data. Found1 (correct data): %t, Found2 (correct data): %t", found1, found2)
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

// TestArticleCommandHandler_HandleUpdateArticlePrice is the new test function for price updates.
// It mirrors the structure of the adapted TestArticleCommandHandler_HandleUpdateArticle (which now tests title updates).
func TestArticleCommandHandler_HandleUpdateArticlePrice(t *testing.T) {
	mockES := &MockEventStore{}
	mockEH := &MockArticleEventHandler{}
	cmdHandler := handlers.NewArticleCommandHandler(mockES, mockEH)

	articleID := newID()
	initialTitle := "Initial Title for Price Test"
	initialContent := "Initial Content for Price Test"
	initialPrice := 20.00
	initialVersion := 0

	initialCreateEventForPriceUpdate := newArticleCreatedEventForTest(articleID, initialTitle, initialContent, initialPrice, initialVersion)

	t.Run("Success_UpdatePrice", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()

		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			if id != articleID {
				t.Fatalf("GetEventsForAggregate called with wrong ID. Expected %s, got %s", articleID, id)
			}
			return []interface{}{initialCreateEventForPriceUpdate}, nil
		}

		newPrice := 25.50
		priceCmd := commands.UpdateArticlePriceCommand{
			ID:    articleID,
			Price: newPrice,
		}

		err := cmdHandler.HandleUpdateArticle(priceCmd) // Using the main dispatcher
		if err != nil {
			t.Fatalf("HandleUpdateArticle(UpdateArticlePriceCommand) failed: %v", err)
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

		priceUpdatedEvent, ok := mockES.SavedEvents[0].(*events.ArticlePriceUpdatedEvent)
		if !ok {
			t.Fatalf("expected ArticlePriceUpdatedEvent, got %T", mockES.SavedEvents[0])
		}
		if priceUpdatedEvent.ID != priceCmd.ID || priceUpdatedEvent.Price != priceCmd.Price {
			t.Errorf("event content mismatch. Expected ID %s, Price %f. Got ID %s, Price %f",
				priceCmd.ID, priceCmd.Price, priceUpdatedEvent.ID, priceUpdatedEvent.Price)
		}
		expectedNewVersion := initialVersion + 1
		if priceUpdatedEvent.Version != expectedNewVersion {
			t.Errorf("expected priceUpdatedEvent.Version to be %d, got %d", expectedNewVersion, priceUpdatedEvent.Version)
		}
		if mockES.SavedExpectedVersion != initialVersion {
			t.Errorf("expected SavedExpectedVersion %d, got %d", initialVersion, mockES.SavedExpectedVersion)
		}

		if !mockEH.HandleEventCalled {
			t.Error("expected EventHandler.HandleEvent to be called")
		}
		if len(mockEH.HandledEvents) != 1 {
			t.Fatalf("expected 1 handled event, got %d", len(mockEH.HandledEvents))
		}
		if mockEH.HandledEvents[0] != priceUpdatedEvent {
			t.Error("event handler handled a different event instance")
		}
	})

	t.Run("AggregateNotFound_UpdatePrice", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		notFoundAggID := newID()
		// notFoundErrText := fmt.Sprintf("aggregat %s nicht gefunden: keine Events vorhanden", notFoundAggID) // This variable is unused
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			if id != notFoundAggID {
				t.Fatalf("GetEventsForAggregate called with wrong ID. Expected %s, got %s", notFoundAggID, id)
			}
			return []interface{}{}, nil // Simulate not found
		}

		priceCmd := commands.UpdateArticlePriceCommand{ID: notFoundAggID, Price: 30.00}
		err := cmdHandler.HandleUpdateArticle(priceCmd)

		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		// The specific error from aggregate.Rehydrate is "aggregat %s nicht gefunden: keine Events vorhanden"
		expectedLoadErr := fmt.Sprintf("aggregat %s nicht gefunden: keine Events vorhanden", notFoundAggID)
		expectedWrappedError := fmt.Sprintf("fehler beim Laden des Aggregats %s für UpdateArticlePriceCommand: %s", notFoundAggID, expectedLoadErr)
		if err.Error() != expectedWrappedError {
			t.Errorf("expected error message '%s', got '%s'", expectedWrappedError, err.Error())
		}
		if !mockES.GetEventsForAggCalled {
			t.Error("expected GetEventsForAggregate to be called")
		}
		if mockES.SaveEventsCalled {
			t.Error("expected SaveEvents NOT to be called")
		}
		if mockEH.HandleEventCalled {
			t.Error("expected HandleEvent NOT to be called")
		}
	})

	t.Run("OptimisticLockError_UpdatePrice", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()

		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{initialCreateEventForPriceUpdate}, nil // Loaded agg version is 0
		}
		optimisticLockErr := errors.New("optimistic lock error: version mismatch")
		mockES.SaveEventsFunc = func(aggregateID string, evts []interface{}, expectedVersion int) error {
			return optimisticLockErr
		}

		priceCmd := commands.UpdateArticlePriceCommand{ID: articleID, Price: 35.00}
		err := cmdHandler.HandleUpdateArticle(priceCmd)

		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		expectedWrappedError := fmt.Sprintf("fehler beim Speichern der Events für Aggregat %s (erwartete Version %d): %s", articleID, initialVersion, optimisticLockErr.Error())
		if err.Error() != expectedWrappedError {
			t.Errorf("expected error message '%s', got '%s'", expectedWrappedError, err.Error())
		}
		if !mockES.GetEventsForAggCalled {
			t.Error("expected GetEventsForAggregate to be called")
		}
		if !mockES.SaveEventsCalled {
			t.Error("expected SaveEvents to be called")
		}
		if mockEH.HandleEventCalled {
			t.Error("expected HandleEvent NOT to be called")
		}
	})

	t.Run("SaveEventsError_UpdatePrice", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{initialCreateEventForPriceUpdate}, nil
		}
		saveErr := errors.New("disk full")
		mockES.SaveEventsFunc = func(aggregateID string, evts []interface{}, expectedVersion int) error {
			return saveErr
		}

		priceCmd := commands.UpdateArticlePriceCommand{ID: articleID, Price: 40.00}
		err := cmdHandler.HandleUpdateArticle(priceCmd)
		if err == nil {
			t.Fatal("expected error from SaveEvents, got nil")
		}
		expectedWrappedError := fmt.Sprintf("fehler beim Speichern der Events für Aggregat %s (erwartete Version %d): %s", articleID, initialVersion, saveErr.Error())
		if err.Error() != expectedWrappedError {
			t.Errorf("expected error '%s', got '%s'", expectedWrappedError, err.Error())
		}
		if mockEH.HandleEventCalled {
			t.Error("EventHandler.HandleEvent should not be called if SaveEvents fails")
		}
	})

	// This test verifies that even if the event handler fails after successful save, the command itself is considered successful.
	t.Run("EventHandlerErrorOnSave_UpdatePrice", func(t *testing.T) {
		mockES.Reset()
		mockEH.Reset()
		mockES.GetEventsForAggregateFunc = func(id string) ([]interface{}, error) {
			return []interface{}{initialCreateEventForPriceUpdate}, nil
		}
		eventHandlerErr := errors.New("read model update failed")
		mockEH.HandleEventFunc = func(event interface{}) error {
			// Simulate error only for ArticlePriceUpdatedEvent
			if _, ok := event.(*events.ArticlePriceUpdatedEvent); ok {
				return eventHandlerErr
			}
			return nil
		}

		priceCmd := commands.UpdateArticlePriceCommand{ID: articleID, Price: 45.00}
		err := cmdHandler.HandleUpdateArticle(priceCmd)
		if err != nil { // The command itself should succeed. The error is logged by the event handler.
			t.Fatalf("HandleUpdateArticle(UpdateArticlePriceCommand) returned an error: %v. Expected nil as event handler errors are logged.", err)
		}
		if !mockES.SaveEventsCalled {
			t.Error("SaveEvents should have been called")
		}
		if !mockEH.HandleEventCalled {
			t.Error("EventHandler.HandleEvent should have been called")
		}
		// Further checks could involve log verification if a logging mock was injected.
	})
}
