package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	// "time" // Removed: Für eventuelle Timeouts oder Logging

	"article-manager/internal/commands"
	"article-manager/internal/eventstore"
	"article-manager/internal/handlers"
	// "article-manager/internal/readmodels" // Removed: Wird für Antworttypen benötigt in main, nur in Tests

	"github.com/google/uuid"
)

// App bündelt die Handler und Abhängigkeiten der Anwendung.
type App struct {
	commandHandler *handlers.ArticleCommandHandler
	eventHandler   *handlers.ArticleEventHandler
	queryHandler   *handlers.ArticleQueryHandler
	eventStore     eventstore.EventStore
}

// --- HTTP Handler Methoden für App ---

// handleCreateArticle verarbeitet Anfragen zum Erstellen neuer Artikel.
// POST /articles
func (a *App) handleCreateArticle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Nur POST-Methode erlaubt", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Fehler beim Parsen des JSON-Requests: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Title == "" || req.Content == "" {
		http.Error(w, "Titel und Inhalt dürfen nicht leer sein", http.StatusBadRequest)
		return
	}

	articleID := uuid.New().String()
	cmd := commands.CreateArticleCommand{
		ID:      articleID,
		Title:   req.Title,
		Content: req.Content,
	}

	if err := a.commandHandler.HandleCreateArticle(cmd); err != nil {
		log.Printf("Fehler bei HandleCreateArticle Command: %v", err)
		http.Error(w, "Fehler beim Erstellen des Artikels: "+err.Error(), http.StatusInternalServerError)
		return
	}

	articleReadModel, err := a.queryHandler.GetArticleByID(articleID)
	if err != nil {
		log.Printf("Fehler beim Abrufen des ReadModels nach Create: %v", err)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": articleID, "status": "Artikel erstellt, ReadModel wird aktualisiert"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(articleReadModel); err != nil {
		log.Printf("Fehler beim Senden der JSON-Antwort für CreateArticle: %v", err)
	}
}

// handleGetAllArticles verarbeitet Anfragen zum Abrufen aller Artikel.
// GET /articles
func (a *App) handleGetAllArticles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Nur GET-Methode erlaubt", http.StatusMethodNotAllowed)
		return
	}

	articles, err := a.queryHandler.GetAllArticles()
	if err != nil {
		log.Printf("Fehler bei GetAllArticles Query: %v", err)
		http.Error(w, "Fehler beim Abrufen aller Artikel: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(articles); err != nil {
		log.Printf("Fehler beim Senden der JSON-Antwort für GetAllArticles: %v", err)
	}
}

// handleGetArticleByID verarbeitet Anfragen zum Abrufen eines einzelnen Artikels.
// GET /articles/{id}
func (a *App) handleGetArticleByID(w http.ResponseWriter, r *http.Request, id string) {
	// Methode wird im Dispatcher geprüft
	article, err := a.queryHandler.GetArticleByID(id)
	if err != nil {
		log.Printf("Fehler bei GetArticleByID Query für ID %s: %v", id, err)
		if strings.Contains(err.Error(), "nicht gefunden") {
			http.Error(w, "Artikel nicht gefunden", http.StatusNotFound)
		} else {
			http.Error(w, "Fehler beim Abrufen des Artikels: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(article); err != nil {
		log.Printf("Fehler beim Senden der JSON-Antwort für GetArticleByID: %v", err)
	}
}

// handleUpdateArticleTitle aktualisiert den Titel eines Artikels.
// PUT/PATCH /articles/{id}/title
// Request Body: {"title": "neuer Titel"}
func (a *App) handleUpdateArticleTitle(w http.ResponseWriter, r *http.Request, id string) {
	// Methode (PUT/PATCH) wird im Dispatcher geprüft
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Fehler beim Parsen des JSON-Requests: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, "Titel darf nicht leer sein", http.StatusBadRequest)
		return
	}

	cmd := commands.UpdateArticleTitleCommand{ID: id, Title: req.Title}
	if err := a.commandHandler.HandleUpdateArticleTitle(cmd); err != nil {
		log.Printf("Fehler bei HandleUpdateArticleTitle Command für ID %s: %v", id, err)
		if strings.Contains(err.Error(), "nicht gefunden") {
			http.Error(w, "Fehler beim Aktualisieren des Titels: Artikel nicht gefunden", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "Titel ist identisch") {
			http.Error(w, "Fehler beim Aktualisieren des Titels: "+err.Error(), http.StatusBadRequest)
		} else if strings.Contains(err.Error(), "optimistic lock error") {
			http.Error(w, "Fehler beim Aktualisieren des Titels: Konflikt (Optimistic Lock)", http.StatusConflict)
		} else {
			http.Error(w, "Fehler beim Aktualisieren des Titels: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	articleReadModel, err := a.queryHandler.GetArticleByID(id)
	if err != nil {
		log.Printf("Fehler beim Abrufen des ReadModels nach UpdateTitle für ID %s: %v", id, err)
		http.Error(w, "Titel aktualisiert, aber Fehler beim Abrufen des ReadModels: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(articleReadModel)
}

// handleUpdateArticleContent aktualisiert den Inhalt eines Artikels.
// PUT/PATCH /articles/{id}/content
// Request Body: {"content": "neuer Inhalt"}
func (a *App) handleUpdateArticleContent(w http.ResponseWriter, r *http.Request, id string) {
	// Methode (PUT/PATCH) wird im Dispatcher geprüft
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Fehler beim Parsen des JSON-Requests: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, "Inhalt darf nicht leer sein", http.StatusBadRequest)
		return
	}

	cmd := commands.UpdateArticleContentCommand{ID: id, Content: req.Content}
	if err := a.commandHandler.HandleUpdateArticleContent(cmd); err != nil {
		log.Printf("Fehler bei HandleUpdateArticleContent Command für ID %s: %v", id, err)
		if strings.Contains(err.Error(), "nicht gefunden") {
			http.Error(w, "Fehler beim Aktualisieren des Inhalts: Artikel nicht gefunden", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "Inhalt ist identisch") {
			http.Error(w, "Fehler beim Aktualisieren des Inhalts: "+err.Error(), http.StatusBadRequest)
		} else if strings.Contains(err.Error(), "optimistic lock error") {
			http.Error(w, "Fehler beim Aktualisieren des Inhalts: Konflikt (Optimistic Lock)", http.StatusConflict)
		} else {
			http.Error(w, "Fehler beim Aktualisieren des Inhalts: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	articleReadModel, err := a.queryHandler.GetArticleByID(id)
	if err != nil {
		log.Printf("Fehler beim Abrufen des ReadModels nach UpdateContent für ID %s: %v", id, err)
		http.Error(w, "Inhalt aktualisiert, aber Fehler beim Abrufen des ReadModels: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(articleReadModel)
}

// handleDeleteArticle verarbeitet Anfragen zum Löschen von Artikeln.
// DELETE /articles/{id}
func (a *App) handleDeleteArticle(w http.ResponseWriter, r *http.Request, id string) {
	// Methode wird im Dispatcher geprüft
	cmd := commands.DeleteArticleCommand{ID: id}
	if err := a.commandHandler.HandleDeleteArticle(cmd); err != nil {
		log.Printf("Fehler bei HandleDeleteArticle Command für ID %s: %v", id, err)
		if strings.Contains(err.Error(), "nicht gefunden") {
			http.Error(w, "Fehler beim Löschen: Artikel nicht gefunden", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "optimistic lock error") {
			http.Error(w, "Fehler beim Löschen: Konflikt (Optimistic Lock)", http.StatusConflict)
		} else {
			http.Error(w, "Fehler beim Löschen des Artikels: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// routeArticleDetailRequests leitet Anfragen für /articles/{id} und /articles/{id}/resource weiter.
func (a *App) routeArticleDetailRequests(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/articles/")
	parts := strings.SplitN(path, "/", 2) // Split into max 2 parts: ID and optional sub-resource
	id := parts[0]

	if id == "" {
		http.NotFound(w, r) // Should not happen if registered for /articles/
		return
	}

	if len(parts) == 1 { // Path ist /articles/{id}
		switch r.Method {
		case http.MethodGet:
			a.handleGetArticleByID(w, r, id)
		case http.MethodDelete:
			a.handleDeleteArticle(w, r, id)
		default:
			http.Error(w, "Methode nicht erlaubt für /articles/{id}", http.StatusMethodNotAllowed)
		}
	} else if len(parts) == 2 { // Path ist /articles/{id}/resource
		resource := parts[1]
		if resource == "title" && (r.Method == http.MethodPut || r.Method == http.MethodPatch) {
			a.handleUpdateArticleTitle(w, r, id)
		} else if resource == "content" && (r.Method == http.MethodPut || r.Method == http.MethodPatch) {
			a.handleUpdateArticleContent(w, r, id)
		} else {
			http.NotFound(w, r)
		}
	} else {
		http.NotFound(w, r) // Sollte durch SplitN(..., 2) nicht vorkommen
	}
}

func main() {
	eventStore := eventstore.NewInMemoryEventStore()
	eventHandler := handlers.NewArticleEventHandler()
	commandHandler := handlers.NewArticleCommandHandler(eventStore, eventHandler)
	queryHandler := handlers.NewArticleQueryHandler(eventHandler)

	app := &App{
		commandHandler: commandHandler,
		eventHandler:   eventHandler,
		queryHandler:   queryHandler,
		eventStore:     eventStore,
	}

	mux := http.NewServeMux()

	// Handler für /articles (ohne ID)
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

	// Handler für /articles/{id}... Routen
	mux.HandleFunc("/articles/", app.routeArticleDetailRequests)

	log.Println("Starte Article-Manager-Server auf Port 8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Fehler beim Starten des HTTP-Servers: %v", err)
	}
}
