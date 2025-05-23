package eventstore

import (
	"errors"
	"sync"
	// "article-manager/internal/events" // Not strictly needed for this basic implementation if we only store interface{}
)

// EventStore definiert die Schnittstelle für die Persistierung und das Abrufen von Events.
type EventStore interface {
	// SaveEvents speichert Events für ein bestimmtes Aggregat.
	// Es erwartet die aggregateID, die Liste der Events und die erwartete Version des Aggregats (für Optimistic Locking).
	SaveEvents(aggregateID string, events []interface{}, expectedVersion int) error
	// GetEventsForAggregate ruft alle Events für ein bestimmtes Aggregat ab.
	GetEventsForAggregate(aggregateID string) ([]interface{}, error)
}

// InMemoryEventStore ist eine einfache In-Memory-Implementierung des EventStore.
// ACHTUNG: Nicht für den Produktionseinsatz geeignet, da die Daten bei Neustart verloren gehen.
type InMemoryEventStore struct {
	// mu schützt den Zugriff auf den events Speicher.
	mu sync.RWMutex
	// events speichert alle Events als Liste von Interfaces pro Aggregat-ID.
	events map[string][]interface{}
	// currentVersion speichert die aktuelle Version jedes Aggregats.
	currentVersion map[string]int
}

// NewInMemoryEventStore erstellt einen neuen InMemoryEventStore.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events:         make(map[string][]interface{}),
		currentVersion: make(map[string]int),
	}
}

// SaveEvents speichert die gegebenen Events.
// Diese Implementierung prüft die erwartete Version für Optimistic Locking.
func (s *InMemoryEventStore) SaveEvents(aggregateID string, newEvents []interface{}, expectedVersion int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentVersion, ok := s.currentVersion[aggregateID]
	if !ok {
		currentVersion = -1 // Aggregat existiert noch nicht, Version ist -1 (vor dem ersten Event)
	}

	if currentVersion != expectedVersion {
		return errors.New("optimistic lock error: version mismatch")
	}

	s.events[aggregateID] = append(s.events[aggregateID], newEvents...)
	s.currentVersion[aggregateID] = currentVersion + len(newEvents) // Die neue Version ist die alte Version plus die Anzahl der neuen Events

	return nil
}

// GetEventsForAggregate ruft alle Events für eine gegebene Aggregat-ID ab.
func (s *InMemoryEventStore) GetEventsForAggregate(aggregateID string) ([]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	eventList, ok := s.events[aggregateID]
	if !ok {
		// Es ist kein Fehler, wenn für ein Aggregat keine Events vorhanden sind.
		// Ein leeres Slice wird zurückgegeben.
		return []interface{}{}, nil
	}
	// Eine Kopie zurückgeben, um externe Modifikationen der internen Slice zu verhindern,
	// obwohl bei []interface{} die Elemente selbst noch Referenzen sein können.
	// Für eine tiefere Kopie müsste man jedes Element einzeln kopieren.
	// Für diesen Anwendungsfall ist eine flache Kopie der Slice ausreichend.
	eventsToReturn := make([]interface{}, len(eventList))
	copy(eventsToReturn, eventList)
	return eventsToReturn, nil
}
