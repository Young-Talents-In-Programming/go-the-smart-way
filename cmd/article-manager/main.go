package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	//"time" // Für eventuelle Timeouts oder Logging

	"article-manager/internal/commands"
	"article-manager/internal/eventstore"
	"article-manager/internal/handlers"
	//"article-manager/internal/readmodels" // Wird für Antworttypen benötigt

	"github.com/google/uuid"
)

// App bündelt die Handler und Abhängigkeiten der Anwendung.
type App struct {
	commandHandler *handlers.ArticleCommandHandler
	eventHandler   *handlers.ArticleEventHandler // Obwohl nicht direkt in HTTP-Handlern verwendet, ist es Teil der Kernlogik
	queryHandler   *handlers.ArticleQueryHandler
	eventStore     eventstore.EventStore // Nötig für die direkte Event-Weiterleitung in dieser vereinfachten Version
}

// --- HTTP Handler Methoden für App ---

// handleCreateArticle verarbeitet Anfragen zum Erstellen neuer Artikel.
// POST /articles
// Request Body: {"title": "string", "content": "string"}
// Response Body (Success 201): {"id": "uuid", "title": "string", "content": "string", "version": 0}
// Response Body (Error 400/500): {"error": "string"}
func (a *App) handleCreateArticle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Nur POST-Methode erlaubt", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Title   *string  `json:"title"`   // Pointer to distinguish missing from empty
		Content *string  `json:"content"` // Pointer to distinguish missing from empty
		Price   *float64 `json:"price"`   // Pointer to distinguish missing from zero
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Fehler beim Parsen des JSON-Requests: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Title == nil || *req.Title == "" {
		http.Error(w, "Titel ist erforderlich und darf nicht leer sein", http.StatusBadRequest)
		return
	}
	if req.Content == nil || *req.Content == "" {
		http.Error(w, "Inhalt ist erforderlich und darf nicht leer sein", http.StatusBadRequest)
		return
	}
	if req.Price == nil {
		http.Error(w, "Preis ist erforderlich", http.StatusBadRequest)
		return
	}
	if *req.Price < 0.0 {
		http.Error(w, "Preis darf nicht negativ sein", http.StatusBadRequest)
		return
	}

	articleID := uuid.New().String()
	cmd := commands.CreateArticleCommand{
		ID:      articleID,
		Title:   *req.Title,
		Content: *req.Content,
		Price:   *req.Price,
	}

	if err := a.commandHandler.HandleCreateArticle(cmd); err != nil {
		log.Printf("Fehler bei HandleCreateArticle Command: %v", err)
		http.Error(w, "Fehler beim Erstellen des Artikels: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Artikel-ReadModel abrufen, um es in der Antwort zurückzugeben
	// Die Version sollte 0 sein nach der Erstellung.
	articleReadModel, err := a.queryHandler.GetArticleByID(articleID)
	if err != nil {
		log.Printf("Fehler beim Abrufen des ReadModels nach Create: %v", err)
		// Der Artikel wurde erstellt, aber das ReadModel ist nicht sofort verfügbar.
		// Dies ist ein Zustand, der in einem verteilten System auftreten könnte.
		// Hier senden wir eine generische Erfolgsmeldung oder die ID.
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": articleID, "status": "Artikel erstellt, ReadModel wird aktualisiert"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(articleReadModel); err != nil {
		log.Printf("Fehler beim Senden der JSON-Antwort für CreateArticle: %v", err)
	}
}

// handleUpdateArticle verarbeitet Anfragen zum Aktualisieren bestehender Artikel.
// PUT /articles/{id}
// Request Body: {"title": "string", "content": "string"}
// Response Body (Success 200): {"id": "uuid", "title": "string", "content": "string", "version": updated_version}
// Response Body (Error 400/404/500): {"error": "string"}
func (a *App) handleUpdateArticle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Nur PUT-Methode erlaubt", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/articles/")
	if id == "" {
		http.Error(w, "Artikel-ID fehlt in der URL", http.StatusBadRequest)
		return
	}

	var updatePayload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		http.Error(w, "Fehler beim Parsen des JSON-Requests: "+err.Error(), http.StatusBadRequest)
		return
	}

	var cmds []interface{}

	// Validate and prepare commands based on payload
	if titleVal, ok := updatePayload["title"]; ok {
		titleStr, isString := titleVal.(string)
		if !isString {
			http.Error(w, "Titel muss ein String sein", http.StatusBadRequest)
			return
		}
		if titleStr == "" {
			http.Error(w, "Titel darf nicht leer sein, wenn er aktualisiert wird", http.StatusBadRequest)
			return
		}
		cmds = append(cmds, commands.UpdateArticleTitleCommand{ID: id, Title: titleStr})
	}

	if contentVal, ok := updatePayload["content"]; ok {
		contentStr, isString := contentVal.(string)
		if !isString {
			http.Error(w, "Inhalt muss ein String sein", http.StatusBadRequest)
			return
		}
		if contentStr == "" {
			http.Error(w, "Inhalt darf nicht leer sein, wenn er aktualisiert wird", http.StatusBadRequest)
			return
		}
		cmds = append(cmds, commands.UpdateArticleContentCommand{ID: id, Content: contentStr})
	}

	if priceVal, ok := updatePayload["price"]; ok {
		priceFloat, isFloat := priceVal.(float64)
		if !isFloat {
			// Allow integers to be decoded as float64
			priceInt, isInt := priceVal.(int)
			if isInt {
				priceFloat = float64(priceInt)
				isFloat = true
			} else {
				priceInt32, isInt32 := priceVal.(int32)
				if isInt32 {
					priceFloat = float64(priceInt32)
					isFloat = true
				} else {
					priceInt64, isInt64 := priceVal.(int64)
					if isInt64 {
						priceFloat = float64(priceInt64)
						isFloat = true
					}
				}
			}
		}


		if !isFloat {
			http.Error(w, "Preis muss eine gültige Zahl sein", http.StatusBadRequest)
			return
		}
		if priceFloat < 0.0 {
			http.Error(w, "Preis darf nicht negativ sein", http.StatusBadRequest)
			return
		}
		cmds = append(cmds, commands.UpdateArticlePriceCommand{ID: id, Price: priceFloat})
	}
	
	if len(cmds) == 0 {
		// No fields to update were provided, or they didn't pass initial checks (e.g. empty strings were not added)
		// Depending on desired behavior, this could be an error or a no-op.
		// For this exercise, let's consider it a Bad Request if the payload is empty or contains only unsupported fields.
		// However, if the payload has valid fields but they just don't trigger any command (e.g. {"price": 0} if we only allow >0),
		// then no command is generated, and below it will just fetch and return the article.
		// The current tests expect a 400 if the update is invalid (e.g. negative price).
		// If the payload is valid JSON but contains no updatable fields (e.g. {"unknown_field": "value"}),
		// `cmds` will be empty.
		// Let's assume for now that if `cmds` is empty after processing, it's not an error but a no-op.
		// The specific test failures (e.g. sending {"price": -5.0}) should be caught by the explicit checks above.
	}


	for _, cmd := range cmds {
		if err := a.commandHandler.HandleUpdateArticle(cmd); err != nil {
			log.Printf("Fehler bei HandleUpdateArticle Command für ID %s: %v", id, err)
			if strings.Contains(err.Error(), "nicht gefunden") {
				http.Error(w, "Fehler beim Aktualisieren: Artikel nicht gefunden", http.StatusNotFound)
			} else if strings.Contains(err.Error(), "optimistic lock error") {
				http.Error(w, "Fehler beim Aktualisieren: Konflikt (Optimistic Lock)", http.StatusConflict)
			} else {
				http.Error(w, "Fehler beim Aktualisieren des Artikels: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}

	articleReadModel, err := a.queryHandler.GetArticleByID(id)
	if err != nil {
		log.Printf("Fehler beim Abrufen des ReadModels nach Update für ID %s: %v", id, err)
		http.Error(w, "Artikel aktualisiert, aber Fehler beim Abrufen des ReadModels: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(articleReadModel); err != nil {
		log.Printf("Fehler beim Senden der JSON-Antwort für UpdateArticle: %v", err)
	}
}

// handleDeleteArticle verarbeitet Anfragen zum Löschen von Artikeln.
// DELETE /articles/{id}
// Response (Success 204): No Content
// Response (Error 404/500): {"error": "string"}
func (a *App) handleDeleteArticle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Nur DELETE-Methode erlaubt", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/articles/")
	if id == "" {
		http.Error(w, "Artikel-ID fehlt in der URL", http.StatusBadRequest)
		return
	}

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

// handleGetArticleByID verarbeitet Anfragen zum Abrufen eines einzelnen Artikels.
// GET /articles/{id}
// Response Body (Success 200): {"id": "uuid", "title": "string", "content": "string", "version": version}
// Response Body (Error 404/500): {"error": "string"}
func (a *App) handleGetArticleByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Nur GET-Methode erlaubt", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/articles/")
	if id == "" {
		http.Error(w, "Artikel-ID fehlt in der URL", http.StatusBadRequest)
		return
	}

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

// handleGetAllArticles verarbeitet Anfragen zum Abrufen aller Artikel.
// GET /articles
// Response Body (Success 200): [{"id": "uuid", "title": "string", "content": "string", "version": version}, ...]
// Response Body (Error 500): {"error": "string"}
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

func main() {
	// Initialisierung der Komponenten
	eventStore := eventstore.NewInMemoryEventStore()
	eventHandler := handlers.NewArticleEventHandler() // Wird von CommandHandler und QueryHandler verwendet

	// Wichtig: Der CommandHandler erhält jetzt den EventHandler, um Events direkt weiterzuleiten.
	commandHandler := handlers.NewArticleCommandHandler(eventStore, eventHandler)
	queryHandler := handlers.NewArticleQueryHandler(eventHandler)

	app := &App{
		commandHandler: commandHandler,
		eventHandler:   eventHandler, // Für Vollständigkeit, auch wenn HTTP-Handler es nicht direkt nutzen
		queryHandler:   queryHandler,
		eventStore:     eventStore, // Für Vollständigkeit
	}

	// HTTP-Routing
	// Ein einfacher ServeMux wird für dieses Beispiel verwendet.
	// Für komplexere Anwendungen könnte ein Router wie Gorilla Mux oder Chi verwendet werden.
	mux := http.NewServeMux()
	mux.HandleFunc("/articles", func(w http.ResponseWriter, r *http.Request) {
		// Unterscheidung GET (alle) und POST (erstellen) basierend auf der Methode
		if r.Method == http.MethodPost {
			app.handleCreateArticle(w, r)
		} else if r.Method == http.MethodGet {
			app.handleGetAllArticles(w, r)
		} else {
			http.Error(w, "Methode nicht erlaubt für /articles", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/articles/", func(w http.ResponseWriter, r *http.Request) {
		// Unterscheidung GET (einzeln), PUT (aktualisieren), DELETE (löschen)
		// Die ID wird aus dem Pfad extrahiert.
		// Beispiel: /articles/uuid-string
		// r.URL.Path wird sein "/articles/uuid-string"
		// id := strings.TrimPrefix(r.URL.Path, "/articles/")

		// Wir müssen sicherstellen, dass wir nicht "/articles" (ohne Trailing Slash) hier abfangen,
		// wenn es für GET all oder POST create gedacht war.
		// Die Registrierung von "/articles" (ohne Slash) und "/articles/" (mit Slash)
		// im ServeMux kann zu diesem Verhalten führen.
		// Es ist oft besser, spezifischere Pfade zuerst zu registrieren oder einen Router zu verwenden,
		// der explizitere Pfadparameter unterstützt.
		// Für dieses Beispiel: Wenn der Pfad genau "/articles/" ist, ist es unklar.
		// Wenn der Pfad länger ist, z.B. "/articles/some-id", dann ist es eine ID-basierte Operation.

		idPart := strings.TrimPrefix(r.URL.Path, "/articles/")
		if idPart == "" && r.URL.Path == "/articles/" { // Pfad ist genau "/articles/"
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

	log.Println("Starte Article-Manager-Server auf Port 8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Fehler beim Starten des HTTP-Servers: %v", err)
	}
}
