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
	// Ensure NewArticleCommandHandler accepts the EventHandler interface
	ch := handlers.NewArticleCommandHandler(es, eh) 
	qh := handlers.NewArticleQueryHandler(eh)

	return &App{
		commandHandler: ch,
		eventHandler:   eh,
		queryHandler:   qh,
		eventStore:     es,
	}
}

// Helper to setup the main router for testing, reflecting main.go
func setupTestRouter(app *App) *http.ServeMux {
	mux := http.NewServeMux()
	
	// Handler for /articles (POST create, GET all)
	mux.HandleFunc("/articles", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			app.handleCreateArticle(w, r)
		case http.MethodGet:
			app.handleGetAllArticles(w, r)
		default:
			http.Error(w, "Methode nicht erlaubt für /articles", http.StatusMethodNotAllowed)
		}
	})

	// Handler for /articles/{id}... routes, dispatched by routeArticleDetailRequests
	mux.HandleFunc("/articles/", app.routeArticleDetailRequests)
	return mux
}

// Helper to create an article directly for test setup
func createArticleForTest(t *testing.T, app *App, title, content string) readmodels.ArticleReadModel {
	t.Helper()
	articleID := uuid.New().String()
	createCmd := commands.CreateArticleCommand{ID: articleID, Title: title, Content: content}
	
	agg := article.NewArticleAggregate(createCmd.ID)
	if err := agg.HandleCreateArticleCommand(createCmd.ID, createCmd.Title, createCmd.Content); err != nil {
		t.Fatalf("Failed to handle create command for test setup: %v", err)
	}
	
	if err := app.eventStore.SaveEvents(agg.ID, agg.GetChanges(), -1); err != nil {
		t.Fatalf("Failed to save events for test setup: %v", err)
	}
	
	for _, event := range agg.GetChanges() {
		if err := app.eventHandler.HandleEvent(event); err != nil {
			t.Fatalf("Failed to handle event for test setup: %v", err)
		}
	}
	agg.ClearChanges()

	rm, err := app.queryHandler.GetArticleByID(articleID)
	if err != nil {
		t.Fatalf("Failed to get created article for test setup: %v", err)
	}
	return rm
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
	createdArticle := createArticleForTest(t, app, "Title Get", "Content Get")

	req, _ := http.NewRequest("GET", "/articles/"+createdArticle.ID, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
	}

	var fetchedArticle readmodels.ArticleReadModel
	if err := json.NewDecoder(rr.Body).Decode(&fetchedArticle); err != nil {
		t.Fatalf("Could not decode response: %v. Body: %s", err, rr.Body.String())
	}
	if fetchedArticle.ID != createdArticle.ID || fetchedArticle.Title != createdArticle.Title || fetchedArticle.Version != 0 {
		t.Errorf("unexpected article data: got %+v want %+v", fetchedArticle, createdArticle)
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

	article1 := createArticleForTest(t, app, "Article 1", "Content 1")
	article2 := createArticleForTest(t, app, "Article 2", "Content 2")

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
	
	foundMap := make(map[string]bool)
	for _, art := range articles {
		foundMap[art.ID] = true
	}
	if !foundMap[article1.ID] || !foundMap[article2.ID] {
		t.Errorf("expected both articles to be present. Found IDs: %v", foundMap)
	}
}

// --- Update Article Title Tests ---
func TestHandleUpdateArticleTitle(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)
	initialArticle := createArticleForTest(t, app, "Original Title", "Original Content")

	t.Run("Success_PUT", func(t *testing.T) {
		payload := map[string]string{"title": "Updated Title via PUT"}
		jsonPayload, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, "/articles/"+initialArticle.ID+"/title", bytes.NewBuffer(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
		}
		var respModel readmodels.ArticleReadModel
		if err := json.NewDecoder(rr.Body).Decode(&respModel); err != nil {
			t.Fatalf("Could not decode response: %v", err)
		}
		if respModel.Title != payload["title"] || respModel.Content != initialArticle.Content || respModel.Version != initialArticle.Version+1 {
			t.Errorf("unexpected article state. Got: %+v", respModel)
		}
	})
    
	// Assuming the title was updated, version is now initialArticle.Version + 1
    // Create a new article for PATCH to ensure clean state for versioning, or re-fetch.
    // For simplicity, let's use a new article.
    patchArticle := createArticleForTest(t, app, "Patch Original Title", "Patch Original Content")
	t.Run("Success_PATCH", func(t *testing.T) {
		payload := map[string]string{"title": "Updated Title via PATCH"}
		jsonPayload, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPatch, "/articles/"+patchArticle.ID+"/title", bytes.NewBuffer(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
		}
		var respModel readmodels.ArticleReadModel
		if err := json.NewDecoder(rr.Body).Decode(&respModel); err != nil {
			t.Fatalf("Could not decode response: %v", err)
		}
		if respModel.Title != payload["title"] || respModel.Content != patchArticle.Content || respModel.Version != patchArticle.Version+1 {
			t.Errorf("unexpected article state. Got: %+v", respModel)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		payload := map[string]string{"title": "Non Existent Title"}
		jsonPayload, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, "/articles/non-existent-id/title", bytes.NewBuffer(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if status := rr.Code; status != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	badRequestTestCases := []struct{ name, payload, expectedErrorPart string }{
		{"InvalidJSON", "{invalid", "Fehler beim Parsen"},
		{"EmptyTitle", `{"title":""}`, "Titel darf nicht leer sein"},
		{"MissingTitle", `{}`, "Titel darf nicht leer sein"}, // Assuming empty string if missing
	}
	for _, tc := range badRequestTestCases {
		t.Run("BadRequest_"+tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPut, "/articles/"+initialArticle.ID+"/title", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if status := rr.Code; status != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tc.expectedErrorPart) {
				t.Errorf("expected error message to contain '%s', got '%s'", tc.expectedErrorPart, rr.Body.String())
			}
		})
	}
}

// --- Update Article Content Tests ---
func TestHandleUpdateArticleContent(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)
	initialArticle := createArticleForTest(t, app, "Original Title", "Original Content")

	t.Run("Success_PUT", func(t *testing.T) {
		payload := map[string]string{"content": "Updated Content via PUT"}
		jsonPayload, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, "/articles/"+initialArticle.ID+"/content", bytes.NewBuffer(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
		}
		var respModel readmodels.ArticleReadModel
		if err := json.NewDecoder(rr.Body).Decode(&respModel); err != nil {
			t.Fatalf("Could not decode response: %v", err)
		}
		if respModel.Content != payload["content"] || respModel.Title != initialArticle.Title || respModel.Version != initialArticle.Version+1 {
			t.Errorf("unexpected article state. Got: %+v", respModel)
		}
	})
    
    patchArticle := createArticleForTest(t, app, "Patch Original Title", "Patch Original Content")
	t.Run("Success_PATCH", func(t *testing.T) {
		payload := map[string]string{"content": "Updated Content via PATCH"}
		jsonPayload, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPatch, "/articles/"+patchArticle.ID+"/content", bytes.NewBuffer(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
		}
		var respModel readmodels.ArticleReadModel
		if err := json.NewDecoder(rr.Body).Decode(&respModel); err != nil {
			t.Fatalf("Could not decode response: %v", err)
		}
		if respModel.Content != payload["content"] || respModel.Title != patchArticle.Title || respModel.Version != patchArticle.Version+1 {
			t.Errorf("unexpected article state. Got: %+v", respModel)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		payload := map[string]string{"content": "Non Existent Content"}
		jsonPayload, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, "/articles/non-existent-id/content", bytes.NewBuffer(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if status := rr.Code; status != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	badRequestTestCases := []struct{ name, payload, expectedErrorPart string }{
		{"InvalidJSON", "{invalid", "Fehler beim Parsen"},
		{"EmptyContent", `{"content":""}`, "Inhalt darf nicht leer sein"},
		{"MissingContent", `{}`, "Inhalt darf nicht leer sein"},
	}
	for _, tc := range badRequestTestCases {
		t.Run("BadRequest_"+tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPut, "/articles/"+initialArticle.ID+"/content", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if status := rr.Code; status != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tc.expectedErrorPart) {
				t.Errorf("expected error message to contain '%s', got '%s'", tc.expectedErrorPart, rr.Body.String())
			}
		})
	}
}

// --- Delete Article Tests ---
func TestHandleDeleteArticle_Success(t *testing.T) {
	app := setupTestApp()
	router := setupTestRouter(app)
	createdArticle := createArticleForTest(t, app, "To Be Deleted", "Delete Me")

	req, _ := http.NewRequest("DELETE", "/articles/"+createdArticle.ID, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusNoContent, rr.Body.String())
	}

	// Verify it's gone
	_, err := app.queryHandler.GetArticleByID(createdArticle.ID)
	if err == nil {
		t.Error("expected error when getting deleted article, but got nil")
	}
    expectedErrStr := fmt.Sprintf("artikel mit ID %s nicht gefunden", createdArticle.ID)
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

// --- Test Method Not Allowed and Routing ---
func TestRoutingAndMethodNotAllowed(t *testing.T) {
    app := setupTestApp()
    router := setupTestRouter(app)
	existingArticle := createArticleForTest(t, app, "Test", "Test")


    testCases := []struct {
        method       string
        path         string
        expectedCode int
    }{
        // Collection endpoint
        {http.MethodPut, "/articles", http.StatusMethodNotAllowed},
        {http.MethodDelete, "/articles", http.StatusMethodNotAllowed},
        {http.MethodPatch, "/articles", http.StatusMethodNotAllowed},

        // Specific resource endpoint /articles/{id}
        {http.MethodPost, "/articles/" + existingArticle.ID, http.StatusMethodNotAllowed},
        {http.MethodPut, "/articles/" + existingArticle.ID, http.StatusMethodNotAllowed}, // No longer general PUT
        {http.MethodPatch, "/articles/" + existingArticle.ID, http.StatusMethodNotAllowed},// No longer general PATCH

        // Title sub-resource
        {http.MethodGet, "/articles/" + existingArticle.ID + "/title", http.StatusNotFound}, // GET not defined for title
        {http.MethodPost, "/articles/" + existingArticle.ID + "/title", http.StatusNotFound},// POST not defined for title
		{http.MethodDelete, "/articles/" + existingArticle.ID + "/title", http.StatusNotFound},// DELETE not defined for title

        // Content sub-resource
        {http.MethodGet, "/articles/" + existingArticle.ID + "/content", http.StatusNotFound},
        {http.MethodPost, "/articles/" + existingArticle.ID + "/content", http.StatusNotFound},
		{http.MethodDelete, "/articles/" + existingArticle.ID + "/content", http.StatusNotFound},

		// Invalid sub-resource
		{http.MethodGet, "/articles/" + existingArticle.ID + "/invalid", http.StatusNotFound},
		{http.MethodPut, "/articles/" + existingArticle.ID + "/invalid", http.StatusNotFound},

    }

    for _, tc := range testCases {
        t.Run(tc.method+"_on_"+strings.ReplaceAll(tc.path, "/", "_"), func(t *testing.T) {
            req, _ := http.NewRequest(tc.method, tc.path, nil)
            rr := httptest.NewRecorder()
            router.ServeHTTP(rr, req)
            if status := rr.Code; status != tc.expectedCode {
                t.Errorf("handler returned wrong status code: got %v want %v for %s %s. Body: %s",
                    status, tc.expectedCode, tc.method, tc.path, rr.Body.String())
            }
        })
    }
}
