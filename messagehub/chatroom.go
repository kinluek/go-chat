package messagehub

import (
	"time"

	"github.com/pkg/errors"
)

var (
	// ErrClosed is returned when an operation is performed on a closed ChatRoom.
	ErrClosed = errors.Errorf("cannot perform operation on closed chatroom")
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

// Event is what clients in the chatroom receive when an event happens.
// This could either be a message, someone joining or leaving the chat room.
type Event struct {
	ID       int
	Type     string
	UserID   string // UserID will be empty for Type "close"
	Message  []byte // Message will be non-nil for Type "message"
	UnixTime int64
}

type eventBufferSet struct {
	events  []Event
	head    int
	tail    int
	isEmpty bool
}

type request struct {
	event         Event
	eventStream   chan<- Event
	catchUpEvents chan eventBufferSet
	done          chan struct{}
}

// ChatRoom represents a chat room and handles the message passing between clients
type ChatRoom struct {
	id         string
	clients    map[string]chan<- Event // key = userid
	requests   chan request
	eventCount int

	// buffer holds the last n (bufferSize) events in memory
	// the buffer is updated in a circular fashion for performance
	// so the head and tail of the buffer must be tracked.
	buffer     []Event
	bufferSize int
	bufferHead int
	bufferTail int

	closed   chan struct{}
	closeSig chan struct{}
}

// NewChatRoom creates a new ChatRoom that is ready to accept requests.
// The ChatRoom must be given an id and a bufferSize, the bufferSize defines
// how many events the ChatRoom can hold in memory, these in memory events
// can let new clients joining the ChatRoom efficiently catchup with recent messages.
//
// Max bufferSize is 1024.
// Min bufferSize is 1.
func NewChatRoom(id string, bufferSize int) *ChatRoom {

	// having a minimum buffer size of one means
	// we don't have to add extra logic to handle empty
	// buffers
	if bufferSize < 1 {
		bufferSize = 1
	}
	if bufferSize > 1024 {
		bufferSize = 1024
	}

	chatRoom := &ChatRoom{
		clients:  make(map[string]chan<- Event),
		requests: make(chan request, 1024),

		buffer:     make([]Event, bufferSize),
		bufferSize: bufferSize,

		closed: make(chan struct{}),
	}
	go chatRoom.serve()
	return chatRoom
}

// Message sends a message to the ChatRoom asynchronously.
// Sending on a closed ChatRoom will be a no-op.
func (c *ChatRoom) Message(senderID string, message []byte) {
	event := Event{
		Type:    EventTypeMessage,
		UserID:  senderID,
		Message: message,
	}
	req := request{event: event}

	// The chatroom server may already be closed so we
	// must select over the channels to prevent blocking.
	select {
	case c.requests <- req:
	case <-c.closed:
	}
}

// Join adds a new user to the ChatRoom. The event stream to listen on and any
// recent events are returned.
// The recent events are ordered from oldest to most recent.
func (c *ChatRoom) Join(userID string, streamBuffer int) (<-chan Event, []Event, error) {
	eventStream := make(chan Event, streamBuffer)
	event := Event{
		Type:   EventTypeJoin,
		UserID: userID,
	}
	req := request{
		event:         event,
		eventStream:   eventStream,
		catchUpEvents: make(chan eventBufferSet, 1),
	}

	select {
	case c.requests <- req:
	case <-c.closed:
	}

	// the chatroom server may be closed while the request is in
	// the requests buffer and the done signal may never be received.
	select {
	case set := <-req.catchUpEvents:

		// we sort the buffer here rather than in the serve goroutine
		// as we want to spend less time blocking the server.
		return eventStream, sortEventsBuffer(set.events, set.head, set.tail), nil
	case <-c.closed:
		return nil, nil, ErrClosed
	}
}

// Leave removes a user from the ChatRoom.
func (c *ChatRoom) Leave(userID string) error {
	event := Event{
		Type:   EventTypeLeave,
		UserID: userID,
	}
	req := request{
		event: event,
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

// Close closes the the ChatRoom.
func (c *ChatRoom) Close() error {
	event := Event{
		Type: EventTypeClose,
	}
	req := request{
		event: event,
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
// through the ChatRoom.
func (c *ChatRoom) serve() {
	for req := range c.requests {
		c.eventCount++

		event := req.event
		event.ID = c.eventCount
		event.UnixTime = time.Now().Unix()

		c.updateBuffer(event)

		// handle request based on event type.
		switch event.Type {
		case EventTypeMessage:
			c.publishEvent(event)
		case EventTypeJoin:
			eventSet := eventBufferSet{
				events: make([]Event, c.bufferSize),
				head:   c.bufferHead,
				tail:   c.bufferTail,
			}
			copy(eventSet.events, c.buffer)
			if _, ok := c.clients[event.UserID]; !ok {
				c.clients[event.UserID] = req.eventStream
				req.catchUpEvents <- eventSet
				c.publishEvent(event)
				continue
			}
			c.clients[event.UserID] = req.eventStream
			req.catchUpEvents <- eventSet
		case EventTypeLeave:
			if _, ok := c.clients[event.UserID]; ok {
				delete(c.clients, event.UserID)
				close(req.done)
				c.publishEvent(event)
				continue
			}
			close(req.done)
		case EventTypeClose:
			c.publishEvent(event)
			for userid := range c.clients {
				delete(c.clients, userid)
			}
			close(c.closed)
			return
		}
	}
}

// publishEvent publishes the provided event to all clients in the chat.
func (c *ChatRoom) publishEvent(event Event) {
	for _, stream := range c.clients {
		select {
		case stream <- event:
		default:
			// events are dropped if no client is
			// actively listening to their eventStream
			// this stops the entire ChatRoom server
			// from blocking.
		}
	}
}

// updateBuffer updates the buffer in a circular fashion.
func (c *ChatRoom) updateBuffer(event Event) {
	if c.eventCount > 1 {
		c.bufferHead++
		if c.bufferHead == c.bufferSize {
			c.bufferHead = 0
		}
		if c.eventCount > c.bufferSize {
			c.bufferTail++
		}
		if c.bufferTail == c.bufferSize {
			c.bufferTail = 0
		}
	}
	c.buffer[c.bufferHead] = event
}

// sortEventsBuffer takes a circular events buffer with knowledge of where
// the head and tail is and reorders the events from oldest to newest.
// Any empty elements are removed.
func sortEventsBuffer(events []Event, head, tail int) []Event {
	if len(events) == 0 {
		return events
	}

	i := 0
	size := len(events)
	sorted := make([]Event, size)
	for {
		sorted[i] = events[tail]
		if tail == head {
			break
		}
		i++
		tail++
		if tail == size {
			tail = 0
		}
	}
	return sorted[:i+1]
}
