package article_test

import (
	"testing"
	"time"
	"errors" // For comparing error messages if needed

	"article-manager/internal/article"
	"article-manager/internal/events"

	"github.com/google/uuid"
)

// Helper function to create a new UUID string for tests
func newID() string {
	return uuid.New().String()
}

func TestNewArticleAggregate(t *testing.T) {
	id := newID()
	agg := article.NewArticleAggregate(id)

	if agg.ID != id {
		t.Errorf("expected ID %s, got %s", id, agg.ID)
	}
	if agg.Version != -1 {
		t.Errorf("expected Version -1, got %d", agg.Version)
	}
	if len(agg.GetChanges()) != 0 {
		t.Errorf("expected 0 changes, got %d", len(agg.GetChanges()))
	}
}

func TestArticleAggregate_HandleCreateArticleCommand(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		initialAggID := newID() // ID for NewArticleAggregate, will be overwritten by command
		agg := article.NewArticleAggregate(initialAggID)
		
		articleID := newID()
		title := "Test Title"
		content := "Test Content"

		err := agg.HandleCreateArticleCommand(articleID, title, content)
		if err != nil {
			t.Fatalf("HandleCreateArticleCommand failed: %v", err)
		}

		if agg.ID != articleID {
			t.Errorf("expected aggregate ID %s, got %s", articleID, agg.ID)
		}
		if agg.Title != title {
			t.Errorf("expected aggregate Title %s, got %s", title, agg.Title)
		}
		if agg.Content != content {
			t.Errorf("expected aggregate Content %s, got %s", content, agg.Content)
		}
		if agg.Version != 0 {
			t.Errorf("expected aggregate Version 0, got %d", agg.Version)
		}

		changes := agg.GetChanges()
		if len(changes) != 1 {
			t.Fatalf("expected 1 event, got %d", len(changes))
		}
		event, ok := changes[0].(*events.ArticleCreatedEvent)
		if !ok {
			t.Fatalf("expected ArticleCreatedEvent, got %T", changes[0])
		}
		if event.ID != articleID {
			t.Errorf("expected event ID %s, got %s", articleID, event.ID)
		}
		if event.Title != title {
			t.Errorf("expected event Title %s, got %s", title, event.Title)
		}
		if event.Content != content {
			t.Errorf("expected event Content %s, got %s", content, event.Content)
		}
		if event.Version != 0 { // Version in event should be 0
			t.Errorf("expected event Version 0, got %d", event.Version)
		}
		if event.Timestamp.IsZero() {
			t.Error("expected event Timestamp to be set")
		}
	})

	t.Run("ValidationErrors", func(t *testing.T) {
		testCases := []struct {
			name    string
			id      string
			title   string
			content string
			wantErr error
		}{
			{"EmptyTitle", newID(), "", "Some Content", errors.New("Titel darf nicht leer sein")},
			{"EmptyContent", newID(), "Some Title", "", errors.New("Inhalt darf nicht leer sein")},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				agg := article.NewArticleAggregate(newID())
				err := agg.HandleCreateArticleCommand(tc.id, tc.title, tc.content)
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if err.Error() != tc.wantErr.Error() {
					t.Errorf("expected error message '%s', got '%s'", tc.wantErr.Error(), err.Error())
				}
				if len(agg.GetChanges()) != 0 {
					t.Errorf("expected 0 changes, got %d", len(agg.GetChanges()))
				}
			})
		}
	})
}

func TestArticleAggregate_HandleUpdateArticleCommand(t *testing.T) {
	baseID := newID()
	baseTitle := "Initial Title"
	baseContent := "Initial Content"

	// Helper to create a base aggregate for update tests
	setupAggregate := func() *article.ArticleAggregate {
		agg := article.NewArticleAggregate(baseID)
		// Apply a create event to simulate an existing article
		err := agg.HandleCreateArticleCommand(baseID, baseTitle, baseContent)
		if err != nil {
			t.Fatalf("setup HandleCreateArticleCommand failed: %v", err)
		}
		agg.ClearChanges() // Clear create event changes before testing update
		return agg
	}

	t.Run("Success", func(t *testing.T) {
		agg := setupAggregate()
		originalVersion := agg.Version // Should be 0

		updatedTitle := "Updated Title"
		updatedContent := "Updated Content"

		err := agg.HandleUpdateArticleCommand(updatedTitle, updatedContent)
		if err != nil {
			t.Fatalf("HandleUpdateArticleCommand failed: %v", err)
		}

		if agg.ID != baseID { // ID should not change
			t.Errorf("expected ID %s, got %s", baseID, agg.ID)
		}
		if agg.Title != updatedTitle {
			t.Errorf("expected Title %s, got %s", updatedTitle, agg.Title)
		}
		if agg.Content != updatedContent {
			t.Errorf("expected Content %s, got %s", updatedContent, agg.Content)
		}
		if agg.Version != originalVersion+1 {
			t.Errorf("expected Version %d, got %d", originalVersion+1, agg.Version)
		}

		changes := agg.GetChanges()
		if len(changes) != 1 {
			t.Fatalf("expected 1 event, got %d", len(changes))
		}
		event, ok := changes[0].(*events.ArticleUpdatedEvent)
		if !ok {
			t.Fatalf("expected ArticleUpdatedEvent, got %T", changes[0])
		}
		if event.ID != baseID {
			t.Errorf("expected event ID %s, got %s", baseID, event.ID)
		}
		if event.Title != updatedTitle {
			t.Errorf("expected event Title %s, got %s", updatedTitle, event.Title)
		}
		if event.Content != updatedContent {
			t.Errorf("expected event Content %s, got %s", updatedContent, event.Content)
		}
		if event.Version != agg.Version { // Version in event should match new aggregate version
			t.Errorf("expected event Version %d, got %d", agg.Version, event.Version)
		}
		if event.Timestamp.IsZero() {
			t.Error("expected event Timestamp to be set")
		}
	})

	t.Run("ValidationErrors", func(t *testing.T) {
		testCases := []struct {
			name    string
			title   string
			content string
			wantErr error
		}{
			{"EmptyTitle", "", "Some Content", errors.New("Titel darf nicht leer sein")},
			{"EmptyContent", "Some Title", "", errors.New("Inhalt darf nicht leer sein")},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				agg := setupAggregate()
				err := agg.HandleUpdateArticleCommand(tc.title, tc.content)
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if err.Error() != tc.wantErr.Error() {
					t.Errorf("expected error message '%s', got '%s'", tc.wantErr.Error(), err.Error())
				}
				if len(agg.GetChanges()) != 0 {
					t.Errorf("expected 0 changes after validation error, got %d", len(agg.GetChanges()))
				}
			})
		}
	})

	t.Run("NoChange", func(t *testing.T) {
		agg := setupAggregate()
		err := agg.HandleUpdateArticleCommand(baseTitle, baseContent) // Same title and content
		if err == nil {
			t.Fatalf("expected error for no change, got nil")
		}
		// Current implementation returns "keine Änderungen festgestellt"
		expectedErr := errors.New("keine Änderungen festgestellt")
		if err.Error() != expectedErr.Error() {
			t.Errorf("expected error message '%s', got '%s'", expectedErr.Error(), err.Error())
		}
		if len(agg.GetChanges()) != 0 {
			t.Errorf("expected 0 changes when no actual update, got %d", len(agg.GetChanges()))
		}
	})
}

func TestArticleAggregate_HandleDeleteArticleCommand(t *testing.T) {
	baseID := newID()
	agg := article.NewArticleAggregate(baseID)
	err := agg.HandleCreateArticleCommand(baseID, "Title to Delete", "Content to Delete")
	if err != nil {
		t.Fatalf("setup HandleCreateArticleCommand failed: %v", err)
	}
	agg.ClearChanges()
	originalVersion := agg.Version // Should be 0

	err = agg.HandleDeleteArticleCommand()
	if err != nil {
		t.Fatalf("HandleDeleteArticleCommand failed: %v", err)
	}

	// State changes for delete might be minimal (e.g., a 'deleted' flag if implemented)
	// Here, we primarily check version and event.
	if agg.Version != originalVersion+1 {
		t.Errorf("expected Version %d, got %d", originalVersion+1, agg.Version)
	}

	changes := agg.GetChanges()
	if len(changes) != 1 {
		t.Fatalf("expected 1 event, got %d", len(changes))
	}
	event, ok := changes[0].(*events.ArticleDeletedEvent)
	if !ok {
		t.Fatalf("expected ArticleDeletedEvent, got %T", changes[0])
	}
	if event.ID != baseID {
		t.Errorf("expected event ID %s, got %s", baseID, event.ID)
	}
	if event.Version != agg.Version { // Version in event should match new aggregate version
		t.Errorf("expected event Version %d, got %d", agg.Version, event.Version)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected event Timestamp to be set")
	}
}

func TestArticleAggregate_ApplyEvent(t *testing.T) {
	t.Run("ApplyArticleCreatedEvent", func(t *testing.T) {
		agg := article.NewArticleAggregate(newID()) // Initial ID, will be overwritten by event
		eventID := newID()
		eventTitle := "Created Title"
		eventContent := "Created Content"
		eventVersion := 0 // Event carries version, but ApplyEvent itself shouldn't modify agg.Version

		originalAggVersion := agg.Version // Should be -1

		event := &events.ArticleCreatedEvent{
			ID:        eventID,
			Title:     eventTitle,
			Content:   eventContent,
			Timestamp: time.Now(),
			Version:   eventVersion,
		}
		err := agg.ApplyEvent(event)
		if err != nil {
			t.Fatalf("ApplyEvent(ArticleCreatedEvent) failed: %v", err)
		}

		if agg.ID != eventID {
			t.Errorf("expected ID %s, got %s", eventID, agg.ID)
		}
		if agg.Title != eventTitle {
			t.Errorf("expected Title %s, got %s", eventTitle, agg.Title)
		}
		if agg.Content != eventContent {
			t.Errorf("expected Content %s, got %s", eventContent, agg.Content)
		}
		// ApplyEvent itself does not change the aggregate's version.
		// Command handlers are responsible for setting/incrementing the version.
		if agg.Version != originalAggVersion {
			t.Errorf("expected Version to remain %d after ApplyEvent, got %d", originalAggVersion, agg.Version)
		}
	})

	t.Run("ApplyArticleUpdatedEvent", func(t *testing.T) {
		agg := article.NewArticleAggregate(newID())
		// Simulate existing state by applying a create event first
		agg.ApplyEvent(&events.ArticleCreatedEvent{ID: agg.ID, Title: "Old Title", Content: "Old Content", Version: 0})
		originalAggVersion := agg.Version // Still -1, as ApplyEvent doesn't change it

		updatedTitle := "Updated Title"
		updatedContent := "Updated Content"
		eventVersion := 1 // Event carries version, but ApplyEvent itself shouldn't modify agg.Version

		event := &events.ArticleUpdatedEvent{
			ID:        agg.ID,
			Title:     updatedTitle,
			Content:   updatedContent,
			Timestamp: time.Now(),
			Version:   eventVersion,
		}
		err := agg.ApplyEvent(event)
		if err != nil {
			t.Fatalf("ApplyEvent(ArticleUpdatedEvent) failed: %v", err)
		}

		if agg.Title != updatedTitle {
			t.Errorf("expected Title %s, got %s", updatedTitle, agg.Title)
		}
		if agg.Content != updatedContent {
			t.Errorf("expected Content %s, got %s", updatedContent, agg.Content)
		}
		if agg.Version != originalAggVersion {
			t.Errorf("expected Version to remain %d after ApplyEvent, got %d", originalAggVersion, agg.Version)
		}
	})

	t.Run("ApplyArticleDeletedEvent", func(t *testing.T) {
		agg := article.NewArticleAggregate(newID())
		// Simulate existing state
		agg.ApplyEvent(&events.ArticleCreatedEvent{ID: agg.ID, Title: "Title", Content: "Content", Version: 0})
		originalAggVersion := agg.Version // Still -1

		eventVersion := 1 // Event carries version

		event := &events.ArticleDeletedEvent{
			ID:        agg.ID,
			Timestamp: time.Now(),
			Version:   eventVersion,
		}
		err := agg.ApplyEvent(event)
		if err != nil {
			t.Fatalf("ApplyEvent(ArticleDeletedEvent) failed: %v", err)
		}
		// Verify any state changes if applicable (e.g., a 'deleted' flag)
		// For now, just ensuring no error and version is not changed by ApplyEvent itself.
		if agg.Version != originalAggVersion {
			t.Errorf("expected Version to remain %d after ApplyEvent, got %d", originalAggVersion, agg.Version)
		}
	})

	t.Run("UnknownEvent", func(t *testing.T) {
		agg := article.NewArticleAggregate(newID())
		err := agg.ApplyEvent("not an event type")
		if err == nil {
			t.Fatal("expected error for unknown event type, got nil")
		}
		expectedErr := errors.New("unbekanntes Event-Typ")
		if err.Error() != expectedErr.Error() {
			t.Errorf("expected error message '%s', got '%s'", expectedErr.Error(), err.Error())
		}
	})
}

func TestArticleAggregate_GetClearChanges(t *testing.T) {
	agg := article.NewArticleAggregate(newID())
	articleID := newID()
	
	// Record a change
	err := agg.HandleCreateArticleCommand(articleID, "Test Title", "Test Content")
	if err != nil {
		t.Fatalf("HandleCreateArticleCommand failed: %v", err)
	}

	changes := agg.GetChanges()
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	agg.ClearChanges()
	changes = agg.GetChanges()
	if len(changes) != 0 {
		t.Errorf("expected 0 changes after ClearChanges, got %d", len(changes))
	}
}

func TestArticleAggregate_IncrementVersion(t *testing.T) {
	agg := article.NewArticleAggregate(newID()) // Version is -1
	
	agg.IncrementVersion() // Version becomes 0
	if agg.Version != 0 {
		t.Errorf("expected Version 0 after first increment, got %d", agg.Version)
	}

	agg.IncrementVersion() // Version becomes 1
	if agg.Version != 1 {
		t.Errorf("expected Version 1 after second increment, got %d", agg.Version)
	}
}

func TestArticleAggregate_HandleCreateArticleCommand_SetsEventVersionCorrectly(t *testing.T) {
    agg := article.NewArticleAggregate(newID())
    articleID := newID()
    err := agg.HandleCreateArticleCommand(articleID, "Title", "Content")
    if err != nil {
        t.Fatalf("HandleCreateArticleCommand failed: %v", err)
    }
    // agg.Version is 0 after HandleCreateArticleCommand
    changes := agg.GetChanges()
    event, _ := changes[0].(*events.ArticleCreatedEvent)
    if event.Version != 0 {
        t.Errorf("expected event Version 0, got %d. Aggregate version is %d", event.Version, agg.Version)
    }
}

func TestArticleAggregate_HandleUpdateArticleCommand_SetsEventVersionCorrectly(t *testing.T) {
    agg := article.NewArticleAggregate(newID())
    agg.HandleCreateArticleCommand(agg.ID, "Initial", "Initial")
    agg.ClearChanges() // Version is 0

    err := agg.HandleUpdateArticleCommand("Updated", "Updated")
    if err != nil {
        t.Fatalf("HandleUpdateArticleCommand failed: %v", err)
    }
    // agg.Version is 1 after HandleUpdateArticleCommand
    changes := agg.GetChanges()
    event, _ := changes[0].(*events.ArticleUpdatedEvent)
    if event.Version != 1 {
        t.Errorf("expected event Version 1, got %d. Aggregate version is %d", event.Version, agg.Version)
    }
}

func TestArticleAggregate_HandleDeleteArticleCommand_SetsEventVersionCorrectly(t *testing.T) {
    agg := article.NewArticleAggregate(newID())
    agg.HandleCreateArticleCommand(agg.ID, "Initial", "Initial")
    agg.ClearChanges() // Version is 0

    err := agg.HandleDeleteArticleCommand()
    if err != nil {
        t.Fatalf("HandleDeleteArticleCommand failed: %v", err)
    }
    // agg.Version is 1 after HandleDeleteArticleCommand
    changes := agg.GetChanges()
    event, _ := changes[0].(*events.ArticleDeletedEvent)
    if event.Version != 1 {
        t.Errorf("expected event Version 1, got %d. Aggregate version is %d", event.Version, agg.Version)
    }
}
