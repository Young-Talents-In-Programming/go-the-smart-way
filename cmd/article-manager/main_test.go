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

	payload := map[string]string{"title": "Test API Title", "content": "Test API Content"}
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
		payload map[string]string
	}{
		{"MissingTitle", map[string]string{"content": "Some Content"}},
		{"MissingContent", map[string]string{"title": "Some Title"}},
		{"EmptyTitle", map[string]string{"title": "", "content": "Some Content"}},
		{"EmptyContent", map[string]string{"title": "Some Title", "content": ""}},
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
	cmd := commands.CreateArticleCommand{ID: articleID, Title: "Title Get", Content: "Content Get"}
	
	// Manually create article state
	agg := article.NewArticleAggregate(cmd.ID) // Aggregate version is -1
	_ = agg.HandleCreateArticleCommand(cmd.ID, cmd.Title, cmd.Content) // Aggregate version becomes 0
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
	cmd1 := commands.CreateArticleCommand{ID: articleID1, Title: "Article 1", Content: "Content 1"}
	agg1 := article.NewArticleAggregate(cmd1.ID)
	_ = agg1.HandleCreateArticleCommand(cmd1.ID, cmd1.Title, cmd1.Content)
	_ = app.eventStore.SaveEvents(agg1.ID, agg1.GetChanges(), -1)
	for _, event := range agg1.GetChanges() { _ = app.eventHandler.HandleEvent(event) }
	agg1.ClearChanges()

	articleID2 := uuid.New().String()
	cmd2 := commands.CreateArticleCommand{ID: articleID2, Title: "Article 2", Content: "Content 2"}
	agg2 := article.NewArticleAggregate(cmd2.ID)
	_ = agg2.HandleCreateArticleCommand(cmd2.ID, cmd2.Title, cmd2.Content)
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
	// Basic check for presence
	found1, found2 := false, false
	for _, art := range articles {
		if art.ID == articleID1 { found1 = true }
		if art.ID == articleID2 { found2 = true }
	}
	if !found1 || !found2 {
		t.Errorf("expected both articles to be present. Found1: %t, Found2: %t", found1, found2)
	}
}

// --- Update Article Tests ---
func TestHandleUpdateArticle_Success(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)

	articleID := uuid.New().String()
	// Pre-populate article
	cmdCreate := commands.CreateArticleCommand{ID: articleID, Title: "Original Title", Content: "Original Content"}
	agg := article.NewArticleAggregate(cmdCreate.ID)
	_ = agg.HandleCreateArticleCommand(cmdCreate.ID, cmdCreate.Title, cmdCreate.Content) // version 0
	_ = app.eventStore.SaveEvents(agg.ID, agg.GetChanges(), -1)
	for _, event := range agg.GetChanges() { _ = app.eventHandler.HandleEvent(event) }
	agg.ClearChanges()


	updatePayload := map[string]string{"title": "Updated Title", "content": "Updated Content"}
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
	if updatedArticle.Title != updatePayload["title"] {
		t.Errorf("Title mismatch: got %s", updatedArticle.Title)
	}
	if updatedArticle.Content != updatePayload["content"] {
		t.Errorf("Content mismatch: got %s", updatedArticle.Content)
	}
	if updatedArticle.Version != 1 { // Version should be incremented
		t.Errorf("Version mismatch: got %d want %d", updatedArticle.Version, 1)
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
	cmdCreate := commands.CreateArticleCommand{ID: articleID, Title: "Original Title", Content: "Original Content"}
	agg := article.NewArticleAggregate(cmdCreate.ID); _ = agg.HandleCreateArticleCommand(cmdCreate.ID, cmdCreate.Title, cmdCreate.Content); _ = app.eventStore.SaveEvents(agg.ID, agg.GetChanges(), -1); for _, event := range agg.GetChanges() { _ = app.eventHandler.HandleEvent(event) }; agg.ClearChanges()


    testCases := []struct {
		name    string
		payload map[string]string
	}{
		{"MissingTitle", map[string]string{"content": "Some Content"}},
		{"MissingContent", map[string]string{"title": "Some Title"}},
        {"EmptyTitle", map[string]string{"title": "", "content": "Some Content"}},
		{"EmptyContent", map[string]string{"title": "Some Title", "content": ""}},
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
	cmdCreate := commands.CreateArticleCommand{ID: articleID, Title: "To Be Deleted", Content: "Delete Me"}
	agg := article.NewArticleAggregate(cmdCreate.ID)
	_ = agg.HandleCreateArticleCommand(cmdCreate.ID, cmdCreate.Title, cmdCreate.Content)
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
