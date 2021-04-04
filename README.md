# Go Chat - Message Hub

This project demonstrates how you can build a highly concurrent, thread safe message hub through communication of channels and goroutines rather than sharing memory across threads and locks. The main package of interest is the [messagehub](./backend/messagehub) package which is protocol agnostic. 

The application server and frontend is just to provide an interactive websocket interface for demonstration purposes and should not be taken as an example for websocket application best practices.
## Running the Chat App

Make sure you have docker installed and run `make run` to spin the application up. The frontend client will be served at `localhost:3000` and the backend will be served on port `8080` by default, so make sure these ports are free.

If you are working on windows and you do not have `make` installed then run the docker commands in the makefile directly. 

## Package messagehub

The [messagehub](./backend/messagehub) package provides a thread safe event bus which callers can subscribe to. Any events sent through the message hub are received by all subscribed clients. 

Access to shared memory is funneled into a single goroutine using channels, which removes the need for mutex locks which are traditionally used in other programming languages that causes contention between threads. 

### Usage

#### Creating a new MessageHub instance

The message hub takes takes an ID, a request buffer size and an event history size.
The request buffer determines how many messages can be popped into the message queue before blocking.
The event history size determines how many events the message hub can hold in memory for quick access.

```go
hub := messagehub.New("GoChat", 1024, 1024)
```

#### Subscribing a client

Calling the Add method will return an event channel which can be listened to.
When adding you must provide a `userID` and a `sessionID`, this is so that a single user
may have multiple listeners and this affects the event types that are produced when 
a user is first added and when new sessions are added to an existing user.

```go
eventStream, err := hub.Add("username", "session-id")
if err != nil {
    return err
}

for event := range eventStream {
    // do stuff with the event
    fmt.Print(event)
}

```

#### Removing a client

```go
err := hub.Remove("username", "session-id")
if err != nil {
    return err
}

```

#### Publishing messages

Publishing a message will send a new "message" event to all subscribed clients asynchronously.
Sending a message on a closed hub will simply be a no-op and will not return an error due to it's asynchronous nature.

```go
hub.Message("username", message)
```

#### Closing the message hub

Closing the message hub will return will send out a close event to every client before
closing every subscribed event channel.

```go
err := hub.Close()
if err != nil {
    return err
}

```

#### Get in-memory History

Calling the History() method will return the most recent events that have passed through the message hub. The number of events stored in the hub is defined by the `historySize` parameter when creating
a new `MessageHub` instance.

```go
events := hub.History()
```


#### Intercepting Events

Below is an example of the [datalog](./backend/datalog/listener.go) package which provides an event listener that writes the events to a log file for persistence. This pattern could be replaced with any other datastore.

```go
// Open a new log file for appending events to.
eventLog, err := os.OpenFile("events.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
if err != nil {
    log.Fatal(err)
}
defer eventLog.Close()

wg := sync.WaitGroup{}

// This returns an event channel that can be attached to the message hub.
// All events going through the message hub will be sent to this channel and logged the file for persistence.
listener := datalog.Listener(eventLog, 10, 1024, &wg)

hub.AttachListener(listener)

```

#### Messagehub Events

Subscribed clients will receive all events, not just messages sent via the Message() api. 
The list of event types can be seen below. It is up to the caller to decide what to do with
each event type.

```go

// This is the event struct which the subscribers will receive on their event streams.
type Event struct {
	ID         int    `json:"id"`
	ChatRoomID string `json:"chatroomId"`
	Type       string `json:"type"`
	UserID     string `json:"userId,omitempty"`    // UserID will be empty for Type "close"
	SessionID  string `json:"sessionId,omitempty"` // SessionID will be empty for Type "close" and "message"
	Message    string `json:"message,omitempty"`   // Message will be non-nil for Type "message"
	UnixTime   int64  `json:"unixTime"`
}

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
```
