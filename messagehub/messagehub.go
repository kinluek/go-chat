package messagehub

import (
	"time"

	"github.com/pkg/errors"
)

var (
	// ErrClosed is returned when an operation is performed on a closed MessageHub.
	ErrClosed = errors.Errorf("cannot perform operation on closed messagehub")
)

const (
	// EventTypeMessage is the event type for when a message is sent to the chat.
	EventTypeMessage = "message"

	// EventTypeJoin happens when a client joins the chat.
	EventTypeJoin = "join"

	// EventTypeLeave happens when a client leaves the chat.
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
	UserID     string `json:"userId,omitempty"`  // UserID will be empty for Type "close"
	Message    string `json:"message,omitempty"` // Message will be non-nil for Type "message"
	UnixTime   int64  `json:"unixTime"`
}

// Request are created and sent through the MessageHub, they can be
// intercepted with middleware.
type Request struct {
	Event         Event
	eventStream   chan<- Event
	catchUpEvents chan *eventHistoryBuffer
	done          chan struct{}
}

// MessageHub represents a chat room and handles the message passing between clients
type MessageHub struct {
	id       string
	clients  map[string]chan<- Event // key = userid
	requests chan Request
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
		clients:  make(map[string]chan<- Event),
		requests: make(chan Request, reqBufferSize),

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
	req := Request{Event: event}

	// The messagehub server may already be closed so we
	// must select over the channels to prevent blocking.
	select {
	case c.requests <- req:
	case <-c.closed:
	}
}

// Join adds a new user to the MessageHub. The event stream to listen on and any
// recent events are returned.
// The recent events are ordered from oldest to most recent.
func (c *MessageHub) Join(userID string, streamBuffer int) (<-chan Event, error) {
	eventStream := make(chan Event, streamBuffer)
	event := Event{
		Type:       EventTypeJoin,
		ChatRoomID: c.id,
		UserID:     userID,
	}
	req := Request{
		Event:       event,
		eventStream: eventStream,
		done:        make(chan struct{}),
	}

	select {
	case c.requests <- req:
	case <-c.closed:
	}

	// the messagehub server may be closed while the request is in
	// the requests buffer and the done signal may never be received.
	select {
	case <-req.done:
		return eventStream, nil
	case <-c.closed:
		return nil, ErrClosed
	}
}

// Leave removes a user from the MessageHub.
func (c *MessageHub) Leave(userID string) error {
	event := Event{
		Type:       EventTypeLeave,
		ChatRoomID: c.id,
		UserID:     userID,
	}
	req := Request{
		Event: event,
		done:  make(chan struct{}),
	}
	select {
	case c.requests <- req:
	case <-c.closed:
	}

	select {
	case <-req.done:
		return nil
	case <-c.closed:
		return ErrClosed
	}
}

// Close closes the the MessageHub.
func (c *MessageHub) Close() error {
	event := Event{
		Type:       EventTypeClose,
		ChatRoomID: c.id,
	}
	req := Request{
		Event: event,
		done:  make(chan struct{}),
	}
	select {
	case c.requests <- req:
	case <-c.closed:
	}
	select {
	case <-req.done:
		return nil
	case <-c.closed:
		return ErrClosed
	}
}

// serve is ran in the background to serve and synchronize requests
// through the MessageHub.
func (c *MessageHub) serve() {

	for req := range c.requests {
		c.eventCount++

		// add serial key and timestamp to event.
		event := req.Event
		event.ID = c.eventCount
		event.UnixTime = time.Now().Unix()

		c.history.push(event)

		// handle request based on event type.
		switch event.Type {
		case EventTypeMessage:
			c.broadcast(event)
		case EventTypeJoin:
			// TODO: add multiple clients per userid
			if _, ok := c.clients[event.UserID]; !ok {
				c.clients[event.UserID] = req.eventStream
				close(req.done)
				c.broadcast(event)
				continue
			}
			c.clients[event.UserID] = req.eventStream
			close(req.done)
		case EventTypeLeave:
			if stream, ok := c.clients[event.UserID]; ok {
				delete(c.clients, event.UserID)
				close(stream)
				close(req.done)
				c.broadcast(event)
				continue
			}
			close(req.done)
		case EventTypeClose:
			c.broadcast(event)
			for userid, stream := range c.clients {
				delete(c.clients, userid)
				close(stream)
			}
			if c.listener != nil {
				close(c.listener)
			}
			close(c.closed)
			return
		}
	}
}

// broadcast publishes the provided event to all clients in the chat.
func (c *MessageHub) broadcast(event Event) {
	if c.listener != nil {
		c.listener <- event
	}
	for _, stream := range c.clients {
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
