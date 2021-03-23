package messagehub

const (
	// EventTypeMessage is the event type for when a message is sent to the chat.
	EventTypeMessage = "message"

	// EventTypeJoin happens when a client joins the chat
	EventTypeJoin = "join"

	// EventTypeLeave happens when a client leaves the chat
	EventTypeLeave = "leave"

	// EventTypeClosed happens when the chat is closed
	EventTypeClosed = "closed"
)

// Event is what clients in the chatroom receive when an event happens.
// This could either be a message, someone joining or leaving the chat room.
type Event struct {
	Type    string
	UserID  string
	Message []byte // Message will be non-nil when Type is "message"
}

type request struct {
	event       Event
	eventStream chan<- Event
	done        chan struct{}
}

// ChatRoom represents a chat room and handles the message passing between clients
type ChatRoom struct {
	clients     map[string]chan<- Event // key = userid
	requests    chan request
	eventBuffer []Event
	eventCount  int

	closed   chan struct{}
	closeSig chan struct{}
}

// NewChatRoom creates a new ChatRoom that is ready to accept requests.
func NewChatRoom() *ChatRoom {
	chatRoom := &ChatRoom{
		clients:     make(map[string]chan<- Event),
		requests:    make(chan request, 1024),
		eventBuffer: make([]Event, 0, 1024),
		closed:      make(chan struct{}),
	}
	go chatRoom.serve()
	return chatRoom
}

// Message sends a message to the ChatRoom asynchronously.
func (c *ChatRoom) Message(senderID string, message []byte) {
	event := Event{
		Type:    EventTypeMessage,
		UserID:  senderID,
		Message: message,
	}
	req := request{event: event}

	// the chatroom server may already be closed so we
	// must select over the channels to prevent blocking.
	select {
	case c.requests <- req:
	case <-c.closed:
	}
}

// Join adds a new user to the ChatRoom. The user must provide an event stream
// to listen ChatRoom Events.
func (c *ChatRoom) Join(userID string, eventStream chan<- Event) {
	event := Event{
		Type:   EventTypeJoin,
		UserID: userID,
	}
	req := request{
		event:       event,
		eventStream: eventStream,
		done:        make(chan struct{}),
	}

	select {
	case c.requests <- req:
	case <-c.closed:
	}

	// the chatroom server may be closed while the request is in
	// the requests buffer and the done signal may never be received.
	select {
	case <-req.done:
	case <-c.closed:
	}
}

// Leave removes a user from the ChatRoom.
func (c *ChatRoom) Leave(userID string) {
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
	case <-c.closed:
	}
}

// Close closes the the ChatRoom.
func (c *ChatRoom) Close() {
	c.closeSig <- struct{}{}
	<-c.closed
}

// serve is ran in the background to serve and synchronize requests
// through the ChatRoom.
func (c *ChatRoom) serve() {
	for {
		select {
		case req := <-c.requests:
			c.eventCount++
			c.eventBuffer = append(c.eventBuffer, req.event)

			switch req.event.Type {
			case EventTypeMessage:
				c.publishEvent(req.event)
			case EventTypeJoin:
				if _, ok := c.clients[req.event.UserID]; !ok {
					c.clients[req.event.UserID] = req.eventStream
					close(req.done)
					c.publishEvent(req.event)
					continue
				}
				c.clients[req.event.UserID] = req.eventStream
				close(req.done)
			case EventTypeLeave:
				if _, ok := c.clients[req.event.UserID]; ok {
					delete(c.clients, req.event.UserID)
					close(req.done)
					c.publishEvent(req.event)
					continue
				}
				close(req.done)
			}
		case <-c.closeSig:
			event := Event{Type: EventTypeClosed}
			c.publishEvent(event)
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

// TODO: Add in-memory events buffer for clients to read from when
// reconnecting or missed events.
