// Package messagehub provides a thread safe event bus which callers can subscribe to.
// Any events sent through the message hub are received by all subscribed clients.
package messagehub

import (
	"errors"
	"time"
)

var (
	// ErrClosed is returned when an operation is performed on a closed MessageHub.
	ErrClosed = errors.New("cannot perform operation on closed messagehub")

	// ErrAlreadyExists is returned when trying to add a new session that already exists.
	ErrAlreadyExists = errors.New("session already exists")

	// ErrNotFound is returned when trying to remove a session that does not exist.
	ErrNotFound = errors.New("session not found")
)

const (
	// EventTypeMessage is the event type for when a message is sent to the chat.
	EventTypeMessage = "message"

	// EventTypeAdd happens when a new session is added for an existing user.
	EventTypeAdd = "add"

	// EventTypeJoin happens when a new user is added with their first session
	EventTypeJoin = "join"

	// EventTypeRemove happens when a session is removed for a user but other sessions still exist.
	EventTypeRemove = "remove"

	// EventTypeLeave happens when a user removes their last session.
	EventTypeLeave = "leave"

	// EventTypeClose happens when the chat is closed.
	EventTypeClose = "close"
)

// Event is what clients in the messagehub receive when an event happens.
// This could either be a message, someone joining or leaving the chat room.
type Event struct {
	ID         int    `json:"id"`
	ChatRoomID string `json:"chatroomId"`
	Type       string `json:"type"`
	UserID     string `json:"userId,omitempty"`    // UserID will be empty for Type "close"
	SessionID  string `json:"sessionId,omitempty"` // SessionID will be empty for Type "close" and "message"
	Message    string `json:"message,omitempty"`   // Message will be non-nil for Type "message"
	UnixTime   int64  `json:"unixTime"`
}

type request struct {
	Event         Event
	eventStream   chan<- Event
	catchUpEvents chan *eventHistoryBuffer
	errs          chan error
}

// MessageHub represents a chat room and handles the message passing between clients
type MessageHub struct {
	id string

	// clients holds a set of sessions for each user
	clients  map[string]map[string]chan<- Event
	requests chan request
	listener chan<- Event

	eventCount int

	// history holds the last n (bufferSize) events in memory
	history *eventHistoryBuffer

	closed chan struct{}
}

// New creates a new MessageHub that is ready to accept requests.
// The MessageHub must be given an id, reqBufferSize and a historySize.
// The reqBufferSize determins the size of the requests channel buffer.
// The historySize defines how many events the MessageHub can hold in memory for quick access.
func New(id string, reqBufferSize, historySize int) *MessageHub {
	hub := &MessageHub{
		id:       id,
		clients:  make(map[string]map[string]chan<- Event),
		requests: make(chan request, reqBufferSize),

		history: newEventsHistoryBuffer(historySize),

		closed: make(chan struct{}),
	}
	go hub.serve()
	return hub
}

// AttachListener adds a listener to the messagehub, the listener will receive
// all events that the clients received. This can be used to process events for
// other purposes like storage.
func (c *MessageHub) AttachListener(listener chan<- Event) {

	// we do not need to use any synchronisation primitives here as this
	// should just be a one time operation.
	c.listener = listener
}

// History gets the available in history of events in chronological order.
func (c *MessageHub) History() []Event {
	return c.history.events()
}

// Message sends a message to the MessageHub asynchronously.
// Sending on a closed MessageHub will be a no-op.
func (c *MessageHub) Message(senderID string, message string) {
	event := Event{
		Type:       EventTypeMessage,
		ChatRoomID: c.id,
		UserID:     senderID,
		Message:    message,
	}
	req := request{Event: event}

	// The messagehub server may already be closed so we
	// must select over the channels to prevent blocking.
	select {
	case c.requests <- req:
	case <-c.closed:
	}
}

// Add adds a new user session to the MessageHub. The event stream to listen on is returned.
// There can be multiple sessions per userID.
func (c *MessageHub) Add(userID, sessionID string, streamBuffer int) (<-chan Event, error) {
	eventStream := make(chan Event, streamBuffer)
	event := Event{
		Type:       EventTypeAdd,
		ChatRoomID: c.id,
		UserID:     userID,
		SessionID:  sessionID,
	}
	req := request{
		Event:       event,
		eventStream: eventStream,
		errs:        make(chan error, 1),
	}

	select {
	case c.requests <- req:
	case <-c.closed:
	}

	// the messagehub server may be closed while the request is in
	// the requests buffer and the done signal may never be received.
	select {
	case err := <-req.errs:
		if err != nil {
			return nil, err
		}
		return eventStream, nil
	case <-c.closed:
		return nil, ErrClosed
	}
}

// Remove removes a user session from the MessageHub.
func (c *MessageHub) Remove(userID, sessionID string) error {
	event := Event{
		Type:       EventTypeRemove,
		ChatRoomID: c.id,
		UserID:     userID,
		SessionID:  sessionID,
	}
	req := request{
		Event: event,
		errs:  make(chan error, 1),
	}
	select {
	case c.requests <- req:
	case <-c.closed:
	}

	select {
	case err := <-req.errs:
		return err
	case <-c.closed:
		return ErrClosed
	}
}

// Close closes all the event streams subscribed to the MessageHub before closing
// the MessageHub itself.
func (c *MessageHub) Close() error {
	event := Event{
		Type:       EventTypeClose,
		ChatRoomID: c.id,
	}
	req := request{
		Event: event,
		errs:  make(chan error, 1),
	}
	select {
	case c.requests <- req:
	case <-c.closed:
	}

	select {
	case <-req.errs:
		close(c.closed)
		return nil
	case <-c.closed:
		return ErrClosed
	}
}

// serve is ran in the background to serve and synchronize requests
// through the MessageHub.
func (c *MessageHub) serve() {
	for req := range c.requests {
		// handle request based on event type.
		switch req.Event.Type {
		case EventTypeMessage:
			c.broadcast(req.Event)
		case EventTypeAdd:
			c.add(req)
		case EventTypeRemove:
			c.remove(req)
		case EventTypeClose:
			c.close(req)
			return
		}
	}
}

// broadcast publishes the provided event to all clients in the chat.
// Only events from successful requests should be broadcasted.
func (c *MessageHub) broadcast(event Event) {

	// add serial key and timestamp to event.
	c.eventCount++
	event.ID = c.eventCount
	event.UnixTime = time.Now().Unix()

	c.history.push(event)

	if c.listener != nil {
		c.listener <- event
	}
	for _, sessions := range c.clients {
		for _, stream := range sessions {
			select {
			case stream <- event:
			default:
				// events are dropped if no client is
				// actively listening to their eventStream
				// this stops the entire MessageHub server
				// from blocking.
			}
		}
	}
}

// add handles the add request.
func (c *MessageHub) add(req request) {
	event := req.Event
	sessions, ok := c.clients[event.UserID]
	if !ok {
		event.Type = EventTypeJoin
		sessions := make(map[string]chan<- Event)
		sessions[event.SessionID] = req.eventStream
		c.clients[event.UserID] = sessions
		req.errs <- nil
		c.broadcast(event)
		return
	}
	if _, ok := sessions[event.SessionID]; !ok {
		sessions[event.SessionID] = req.eventStream
		req.errs <- nil
		c.broadcast(event)
		return
	}
	req.errs <- ErrAlreadyExists
}

// remove handles the remove request.
func (c *MessageHub) remove(req request) {
	event := req.Event
	if sessions, ok := c.clients[event.UserID]; ok {
		if stream, ok := sessions[event.SessionID]; ok {
			delete(sessions, event.SessionID)
			close(stream)
			req.errs <- nil
			if len(sessions) == 0 {
				event.Type = EventTypeLeave
				delete(c.clients, event.UserID)
			}
			c.broadcast(event)
			return
		}
	}
	req.errs <- ErrNotFound
}

// close handles the close request.
func (c *MessageHub) close(req request) {
	c.broadcast(req.Event)
	for _, sessions := range c.clients {
		for _, stream := range sessions {
			close(stream)
		}
	}
	if c.listener != nil {
		close(c.listener)
	}
	req.errs <- nil
}
