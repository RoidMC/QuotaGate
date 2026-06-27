package event

import (
	"time"

	"github.com/roidmc/quotagate/internal/types"
)

// EventType is an alias for types.EventType. All event type constants
// are defined in the types package as the single source of truth.
// Use types.ActionUserRegister, types.ActionUserLogin, etc. directly.
type EventType = types.EventType

// Wildcard is a special EventType that matches all events. Handlers
// subscribed with Wildcard will be invoked for every published event,
// in addition to any type-specific handlers.
const Wildcard EventType = "*"

type Event struct {
	ID        string      `json:"id"`
	Type      EventType   `json:"type"`
	Source    string      `json:"source"`
	Subject   string      `json:"subject"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

type EventHandler func(Event)
