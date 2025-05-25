package main

import (
	"article-manager/internal/article" // For direct aggregate manipulation in test setup
	"article-manager/internal/commands"
	"article-manager/internal/eventstore"
	"article-manager/internal/handlers"
	"article-manager/internal/readmodels"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Helper to initialize the App for tests
func setupTestApp() *App {
	es := eventstore.NewInMemoryEventStore()
	eh := handlers.NewArticleEventHandler()
	ch := handlers.NewArticleCommandHandler(es, eh)
	qh := handlers.NewArticleQueryHandler(eh)

	return &App{
		commandHandler: ch,
		eventHandler:   eh,
		queryHandler:   qh,
		eventStore:     es,
	}
}

// Helper to setup the main router for testing
func setupTestRouter(app *App) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/articles", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			app.handleCreateArticle(w, r)
		} else if r.Method == http.MethodGet {
			app.handleGetAllArticles(w, r)
		} else {
			http.Error(w, "Methode nicht erlaubt für /articles", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/articles/", func(w http.ResponseWriter, r *http.Request) {
		idPart := strings.TrimPrefix(r.URL.Path, "/articles/")
		if idPart == "" && r.URL.Path == "/articles/" {
			http.Error(w, "Ungültiger Endpunkt. Meinten Sie /articles oder /articles/{id}?", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			app.handleGetArticleByID(w, r)
		case http.MethodPut:
			app.handleUpdateArticle(w, r)
		case http.MethodDelete:
			app.handleDeleteArticle(w, r)
		default:
			http.Error(w, "Methode nicht erlaubt für /articles/{id}", http.StatusMethodNotAllowed)
		}
	})
	return mux
}

// --- Create Article Tests ---
func TestHandleCreateArticle_Success(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	payload := map[string]interface{}{"title": "Test API Title", "content": "Test API Content", "price": 10.99}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/articles", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusCreated, rr.Body.String())
	}

	var createdArticle readmodels.ArticleReadModel
	if err := json.NewDecoder(rr.Body).Decode(&createdArticle); err != nil {
		t.Fatalf("Could not decode response: %v. Body: %s", err, rr.Body.String())
	}

	if createdArticle.Title != payload["title"] {
		t.Errorf("handler returned unexpected title: got '%s' want '%s'", createdArticle.Title, payload["title"])
	}
	if createdArticle.Content != payload["content"] {
		t.Errorf("handler returned unexpected content: got '%s' want '%s'", createdArticle.Content, payload["content"])
	}
	if createdArticle.Price != payload["price"] {
		t.Errorf("handler returned unexpected price: got '%v' want '%v'", createdArticle.Price, payload["price"])
	}
	if createdArticle.ID == "" {
		t.Error("handler returned empty ID")
	}
	if createdArticle.Version != 0 {
		t.Errorf("handler returned unexpected version: got %d want %d", createdArticle.Version, 0)
	}
}

func TestHandleCreateArticle_BadRequest_InvalidJSON(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	req, _ := http.NewRequest("POST", "/articles", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid JSON: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestHandleCreateArticle_BadRequest_MissingFields(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	testCases := []struct {
		name    string
		payload interface{}
	}{
		{"MissingTitle", map[string]interface{}{"content": "Some Content", "price": 10.99}},
		{"MissingContent", map[string]interface{}{"title": "Some Title", "price": 10.99}},
		{"MissingPrice", map[string]interface{}{"title": "Some Title", "content": "Some Content"}},
		{"EmptyTitle", map[string]interface{}{"title": "", "content": "Some Content", "price": 10.99}},
		{"EmptyContent", map[string]interface{}{"title": "Some Title", "content": "", "price": 10.99}},
		{"NegativePrice", map[string]interface{}{"title": "Some Title", "content": "Some Content", "price": -1.0}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonPayload, _ := json.Marshal(tc.payload)
			req, _ := http.NewRequest("POST", "/articles", bytes.NewBuffer(jsonPayload))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusBadRequest {
				t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusBadRequest, rr.Body.String())
			}
		})
	}
}

// --- Get Article By ID Tests ---
func TestHandleGetArticleByID_Success(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	articleID := uuid.New().String()
	cmd := commands.CreateArticleCommand{ID: articleID, Title: "Title Get", Content: "Content Get", Price: 12.34}

	// Manually create article state
	agg := article.NewArticleAggregate(cmd.ID) // Aggregate version is -1
	_ = agg.HandleCreateArticleCommand(cmd.ID, cmd.Title, cmd.Content, cmd.Price) // Aggregate version becomes 0
	_ = app.eventStore.SaveEvents(agg.ID, agg.GetChanges(), -1) // Store expects -1 for new
	for _, event := range agg.GetChanges() {
		_ = app.eventHandler.HandleEvent(event) // Event handler updates read model
	}
	agg.ClearChanges()

	req, _ := http.NewRequest("GET", "/articles/"+articleID, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
	}

	var fetchedArticle readmodels.ArticleReadModel
	if err := json.NewDecoder(rr.Body).Decode(&fetchedArticle); err != nil {
		t.Fatalf("Could not decode response: %v. Body: %s", err, rr.Body.String())
	}
	if fetchedArticle.ID != articleID {
		t.Errorf("unexpected article ID: got %s want %s", fetchedArticle.ID, articleID)
	}
	if fetchedArticle.Title != cmd.Title {
		t.Errorf("unexpected article Title: got %s want %s", fetchedArticle.Title, cmd.Title)
	}
	if fetchedArticle.Content != cmd.Content { // Added content check for completeness
		t.Errorf("unexpected article Content: got %s want %s", fetchedArticle.Content, cmd.Content)
	}
	if fetchedArticle.Price != cmd.Price {
		t.Errorf("unexpected article Price: got %f want %f", fetchedArticle.Price, cmd.Price)
	}
	if fetchedArticle.Version != 0 {
		t.Errorf("unexpected article Version: got %d want %d", fetchedArticle.Version, 0)
	}
}

func TestHandleGetArticleByID_NotFound(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	req, _ := http.NewRequest("GET", "/articles/non-existent-id", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code for not found: got %v want %v", status, http.StatusNotFound)
	}
}

// --- Get All Articles Tests ---
func TestHandleGetAllArticles_Success_Empty(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	req, _ := http.NewRequest("GET", "/articles", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
	}

	var articles []readmodels.ArticleReadModel
	if err := json.NewDecoder(rr.Body).Decode(&articles); err != nil {
		t.Fatalf("Could not decode response: %v. Body: %s", err, rr.Body.String())
	}
	if len(articles) != 0 {
		t.Errorf("expected 0 articles, got %d", len(articles))
	}
}

func TestHandleGetAllArticles_Success_WithData(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	// Create two articles
	articleID1 := uuid.New().String()
	cmd1 := commands.CreateArticleCommand{ID: articleID1, Title: "Article 1", Content: "Content 1", Price: 5.99}
	agg1 := article.NewArticleAggregate(cmd1.ID)
	_ = agg1.HandleCreateArticleCommand(cmd1.ID, cmd1.Title, cmd1.Content, cmd1.Price)
	_ = app.eventStore.SaveEvents(agg1.ID, agg1.GetChanges(), -1)
	for _, event := range agg1.GetChanges() { _ = app.eventHandler.HandleEvent(event) }
	agg1.ClearChanges()

	articleID2 := uuid.New().String()
	cmd2 := commands.CreateArticleCommand{ID: articleID2, Title: "Article 2", Content: "Content 2", Price: 15.99}
	agg2 := article.NewArticleAggregate(cmd2.ID)
	_ = agg2.HandleCreateArticleCommand(cmd2.ID, cmd2.Title, cmd2.Content, cmd2.Price)
	_ = app.eventStore.SaveEvents(agg2.ID, agg2.GetChanges(), -1)
	for _, event := range agg2.GetChanges() { _ = app.eventHandler.HandleEvent(event) }
	agg2.ClearChanges()

	req, _ := http.NewRequest("GET", "/articles", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
	}

	var articles []readmodels.ArticleReadModel
	if err := json.NewDecoder(rr.Body).Decode(&articles); err != nil {
		t.Fatalf("Could not decode response: %v. Body: %s", err, rr.Body.String())
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}
	// Basic check for presence and data
	articleMap := make(map[string]readmodels.ArticleReadModel)
	for _, art := range articles {
		articleMap[art.ID] = art
	}

	art1, ok1 := articleMap[articleID1]
	if !ok1 {
		t.Errorf("Article with ID %s not found in response", articleID1)
	} else {
		if art1.Title != cmd1.Title {
			t.Errorf("Article 1 Title mismatch: got %s, want %s", art1.Title, cmd1.Title)
		}
		if art1.Price != cmd1.Price {
			t.Errorf("Article 1 Price mismatch: got %f, want %f", art1.Price, cmd1.Price)
		}
	}

	art2, ok2 := articleMap[articleID2]
	if !ok2 {
		t.Errorf("Article with ID %s not found in response", articleID2)
	} else {
		if art2.Title != cmd2.Title {
			t.Errorf("Article 2 Title mismatch: got %s, want %s", art2.Title, cmd2.Title)
		}
		if art2.Price != cmd2.Price {
			t.Errorf("Article 2 Price mismatch: got %f, want %f", art2.Price, cmd2.Price)
		}
	}
}

// --- Update Article Tests ---
func TestHandleUpdateArticle_Success(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	articleID := uuid.New().String()
	initialPrice := 7.77
	// Pre-populate article
	cmdCreate := commands.CreateArticleCommand{ID: articleID, Title: "Original Title", Content: "Original Content", Price: initialPrice}
	agg := article.NewArticleAggregate(cmdCreate.ID)
	_ = agg.HandleCreateArticleCommand(cmdCreate.ID, cmdCreate.Title, cmdCreate.Content, cmdCreate.Price) // version 0
	_ = app.eventStore.SaveEvents(agg.ID, agg.GetChanges(), -1)
	for _, event := range agg.GetChanges() { _ = app.eventHandler.HandleEvent(event) }
	agg.ClearChanges()

	updatedTitle := "Updated Title Completely"
	updatedContent := "Updated Content Completely"
	updatedPrice := 15.99

	// Simulate partial updates by sending only the fields to be changed
	// Based on main.go, the handler expects a map[string]interface{} where keys are field names
	updatePayload := map[string]interface{}{
		"title":   updatedTitle,   // This will generate an UpdateArticleTitleCommand
		"content": updatedContent, // This will generate an UpdateArticleContentCommand
		"price":   updatedPrice,   // This will generate an UpdateArticlePriceCommand
	}
	jsonPayload, _ := json.Marshal(updatePayload)

	req, _ := http.NewRequest("PUT", "/articles/"+articleID, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
	}

	var updatedArticle readmodels.ArticleReadModel
	if err := json.NewDecoder(rr.Body).Decode(&updatedArticle); err != nil {
		t.Fatalf("Could not decode response: %v. Body: %s", err, rr.Body.String())
	}
	if updatedArticle.ID != articleID {
		t.Errorf("ID mismatch: got %s", updatedArticle.ID)
	}
	if updatedArticle.Title != updatedTitle {
		t.Errorf("Title mismatch: got %s, want %s", updatedArticle.Title, updatedTitle)
	}
	if updatedArticle.Content != updatedContent {
		t.Errorf("Content mismatch: got %s, want %s", updatedArticle.Content, updatedContent)
	}
	if updatedArticle.Price != updatedPrice {
		t.Errorf("Price mismatch: got %f, want %f", updatedArticle.Price, updatedPrice)
	}
	// Each command (Title, Content, Price) will increment the version.
	// Initial version is 0.
	// After Title update: version 1
	// After Content update: version 2
	// After Price update: version 3
	expectedVersion := 3
	if updatedArticle.Version != expectedVersion {
		t.Errorf("Version mismatch: got %d want %d", updatedArticle.Version, expectedVersion)
	}
}

func TestHandleUpdateArticle_NotFound(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	updatePayload := map[string]string{"title": "Updated Title", "content": "Updated Content"}
	jsonPayload, _ := json.Marshal(updatePayload)
	req, _ := http.NewRequest("PUT", "/articles/non-existent-id", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusNotFound, rr.Body.String())
	}
}

func TestHandleUpdateArticle_BadRequest_InvalidJSON(t *testing.T) {
    app := setupTestApp()
    router := setupTestRouter(app)
    req, _ := http.NewRequest("PUT", "/articles/some-id", bytes.NewBufferString("{invalid json"))
    req.Header.Set("Content-Type", "application/json")
    rr := httptest.NewRecorder()
    router.ServeHTTP(rr, req)
    if status := rr.Code; status != http.StatusBadRequest {
        t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
    }
}

func TestHandleUpdateArticle_BadRequest_MissingFields(t *testing.T) {
    app := setupTestApp()
    router := setupTestRouter(app)
	articleID := uuid.New().String()
	// Pre-populate article so it exists for update attempt
	cmdCreate := commands.CreateArticleCommand{ID: articleID, Title: "Original Title", Content: "Original Content", Price: 9.99}
	agg := article.NewArticleAggregate(cmdCreate.ID)
	_ = agg.HandleCreateArticleCommand(cmdCreate.ID, cmdCreate.Title, cmdCreate.Content, cmdCreate.Price)
	_ = app.eventStore.SaveEvents(agg.ID, agg.GetChanges(), -1)
	for _, event := range agg.GetChanges() { _ = app.eventHandler.HandleEvent(event) }
	agg.ClearChanges()


    testCases := []struct {
		name    string
		payload map[string]interface{} 
	}{
		// These test individual invalid field updates via the partial update mechanism
        {"EmptyTitleToUpdate", map[string]interface{}{"title": ""}}, 
		{"EmptyContentToUpdate", map[string]interface{}{"content": ""}}, 
		{"NegativePriceToUpdate", map[string]interface{}{"price": -5.0}}, 
		// Test with a valid field and an invalid one to ensure atomicity or error handling
		{"ValidTitleAndNegativePrice", map[string]interface{}{"title": "Good Title", "price": -2.0}},
	}
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            jsonPayload, _ := json.Marshal(tc.payload)
            req, _ := http.NewRequest("PUT", "/articles/"+articleID, bytes.NewBuffer(jsonPayload))
            req.Header.Set("Content-Type", "application/json")
            rr := httptest.NewRecorder()
            router.ServeHTTP(rr, req)
            if status := rr.Code; status != http.StatusBadRequest {
                t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusBadRequest, rr.Body.String())
            }
        })
    }
}


// --- Delete Article Tests ---
func TestHandleDeleteArticle_Success(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	articleID := uuid.New().String()
	// Pre-populate article
	cmdCreate := commands.CreateArticleCommand{ID: articleID, Title: "To Be Deleted", Content: "Delete Me", Price: 1.00}
	agg := article.NewArticleAggregate(cmdCreate.ID)
	_ = agg.HandleCreateArticleCommand(cmdCreate.ID, cmdCreate.Title, cmdCreate.Content, cmdCreate.Price)
	_ = app.eventStore.SaveEvents(agg.ID, agg.GetChanges(), -1)
	for _, event := range agg.GetChanges() { _ = app.eventHandler.HandleEvent(event) }
	agg.ClearChanges()

	req, _ := http.NewRequest("DELETE", "/articles/"+articleID, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusNoContent, rr.Body.String())
	}

	// Verify it's gone
	_, err := app.queryHandler.GetArticleByID(articleID)
	if err == nil {
		t.Error("expected error when getting deleted article, but got nil")
	}
    expectedErrStr := fmt.Sprintf("artikel mit ID %s nicht gefunden", articleID)
    if err.Error() != expectedErrStr {
        t.Errorf("expected error '%s' for deleted article, got '%s'", expectedErrStr, err.Error())
    }
}

func TestHandleDeleteArticle_NotFound(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	req, _ := http.NewRequest("DELETE", "/articles/non-existent-id", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusNotFound, rr.Body.String())
	}
}

// --- Test Method Not Allowed ---
func TestMethodNotAllowed(t *testing.T) {
    app := setupTestApp()
    router := setupTestRouter(app)

    testCases := []struct {
        method string
        path   string
    }{
        {http.MethodPut, "/articles"},      // PUT on /articles
        {http.MethodDelete, "/articles"},   // DELETE on /articles
        {http.MethodPost, "/articles/id"}, // POST on /articles/{id}
    }

    for _, tc := range testCases {
        t.Run(tc.method+"_on_"+strings.ReplaceAll(tc.path, "/", "_"), func(t *testing.T) {
            req, _ := http.NewRequest(tc.method, tc.path, nil)
            rr := httptest.NewRecorder()
            router.ServeHTTP(rr, req)
            if status := rr.Code; status != http.StatusMethodNotAllowed {
                t.Errorf("handler returned wrong status code: got %v want %v for %s %s",
                    status, http.StatusMethodNotAllowed, tc.method, tc.path)
            }
        })
    }
}
