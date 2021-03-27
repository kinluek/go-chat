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
	Message    []byte `json:"message,omitempty"` // Message will be non-nil for Type "message"
	UnixTime   int64  `json:"unixTime"`
}

// Request are created and sent through the MessageHub, they can be
// intercepted with middleware.
type Request struct {
	Event         Event
	eventStream   chan<- Event
	catchUpEvents chan *eventsRingBuffer
	done          chan struct{}
}

// MiddleWare can be attached to the MessageHub, all requests will be streamed
// through the middleware in an orderly fashion. The channel returned by
// the middleware should not be buffered, this ensures that the requests
// are pulled through correctly.
type MiddleWare func(<-chan Request) <-chan Request

// MessageHub represents a chat room and handles the message passing between clients
type MessageHub struct {
	id         string
	clients    map[string]chan<- Event // key = userid
	requests   chan Request
	mids       []MiddleWare
	eventCount int

	// history holds the last n (bufferSize) events in memory
	history *eventsRingBuffer

	closed chan struct{}
}

// New creates a new MessageHub that is ready to accept requests.
// The MessageHub must be given an id and a bufferSize, the bufferSize defines
// how many events the MessageHub can hold in memory, these in memory events
// can let new clients joining the MessageHub efficiently catchup with recent messages.
//
// Max bufferSize is 1024.
// Min bufferSize is 1.
func New(id string, bufferSize int, mids ...MiddleWare) *MessageHub {

	// having a minimum buffer size of one means
	// we don't have to add extra logic to handle empty
	// buffers
	if bufferSize < 1 {
		bufferSize = 1
	}
	if bufferSize > 1024 {
		bufferSize = 1024
	}

	hub := &MessageHub{
		id:       id,
		clients:  make(map[string]chan<- Event),
		requests: make(chan Request, bufferSize),
		mids:     mids,

		history: newEventsRingBuffer(bufferSize),

		closed: make(chan struct{}),
	}
	go hub.serve()
	return hub
}

// Message sends a message to the MessageHub asynchronously.
// Sending on a closed MessageHub will be a no-op.
func (c *MessageHub) Message(senderID string, message []byte) {
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

// History gets the available in history of events in chronological order.
func (c *MessageHub) History() []Event {
	return c.history.events()
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

	// apply generator middleware first
	requestStream := wrapMiddleware(c.requests, c.generator)

	// apply custom middleware
	requestStream = wrapMiddleware(requestStream, c.mids...)

	for req := range requestStream {
		event := req.Event

		c.history.push(event)

		// handle request based on event type.
		switch event.Type {
		case EventTypeMessage:
			c.broadcast(event)
		case EventTypeJoin:
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
			for userid := range c.clients {
				delete(c.clients, userid)
			}
			close(c.closed)
			return
		}
	}
}

// broadcast publishes the provided event to all clients in the chat.
func (c *MessageHub) broadcast(event Event) {
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

// generator is the first middleware that is applied to the request stream
// and applies the sequential id and timestamps to events. It also closes
// the outputted request stream once the MessageHub is closed so that other
// middleware know when to exit.
func (c *MessageHub) generator(in <-chan Request) <-chan Request {
	out := make(chan Request)
	go func() {
		for {
			select {
			case req := <-in:
				c.eventCount++
				req.Event.ID = c.eventCount
				req.Event.UnixTime = time.Now().Unix()
				out <- req
			case <-c.closed:
				close(out)
				return
			}
		}
	}()
	return out
}

func wrapMiddleware(requests <-chan Request, middleware ...MiddleWare) <-chan Request {
	for i := 0; i < len(middleware); i++ {
		mw := middleware[i]
		if mw != nil {
			requests = mw(requests)
		}
	}
	return requests
}
