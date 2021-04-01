package messagehub

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageHub(t *testing.T) {
	hub := New("hub-id", 20, 10)

	// should create "join" event
	user1Events, err := hub.Add("user1", "session1", 10)
	if err != nil {
		t.Fatal(err)
	}

	// should create "join" event
	user2Events1, err := hub.Add("user2", "session1", 10)
	if err != nil {
		t.Fatal(err)
	}

	// should create "add" event as user2 is adding a second session
	user2Events2, err := hub.Add("user2", "session2", 10)
	if err != nil {
		t.Fatal(err)
	}

	// should create "join" event
	user3Events, err := hub.Add("user3", "session1", 10)
	if err != nil {
		t.Fatal(err)
	}

	user1ReceivedEvents := make([]Event, 0)
	user2ReceivedEvents1 := make([]Event, 0)
	user2ReceivedEvents2 := make([]Event, 0)
	user3ReceivedEvents := make([]Event, 0)

	wg := sync.WaitGroup{}
	wg.Add(4)

	go func() {
		defer wg.Done()
		for event := range user1Events {
			user1ReceivedEvents = append(user1ReceivedEvents, event)
		}
	}()

	go func() {
		defer wg.Done()
		for event := range user2Events1 {
			user2ReceivedEvents1 = append(user2ReceivedEvents1, event)
		}
	}()
	go func() {
		defer wg.Done()
		for event := range user2Events2 {
			user2ReceivedEvents2 = append(user2ReceivedEvents2, event)
		}
	}()

	go func() {
		defer wg.Done()
		for event := range user3Events {
			user3ReceivedEvents = append(user3ReceivedEvents, event)
		}
	}()

	hub.Message("send-id", "hello")

	// should create "leave" event
	hub.Remove("user1", "session1")

	// should create "remove" event as user2 still has existing session
	hub.Remove("user2", "session2")
	hub.Message("send-id", "user1 left")
	hub.Close()

	wg.Wait()

	expectedEvents1 := []Event{
		{ID: 1, Type: "join", ChatRoomID: "hub-id", UserID: "user1", SessionID: "session1"},
		{ID: 2, Type: "join", ChatRoomID: "hub-id", UserID: "user2", SessionID: "session1"},
		{ID: 3, Type: "add", ChatRoomID: "hub-id", UserID: "user2", SessionID: "session2"},
		{ID: 4, Type: "join", ChatRoomID: "hub-id", UserID: "user3", SessionID: "session1"},
		{ID: 5, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: "hello"},
	}

	expectedEvents21 := []Event{
		{ID: 2, Type: "join", ChatRoomID: "hub-id", UserID: "user2", SessionID: "session1"},
		{ID: 3, Type: "add", ChatRoomID: "hub-id", UserID: "user2", SessionID: "session2"},
		{ID: 4, Type: "join", ChatRoomID: "hub-id", UserID: "user3", SessionID: "session1"},
		{ID: 5, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: "hello"},
		{ID: 6, Type: "leave", ChatRoomID: "hub-id", UserID: "user1", SessionID: "session1"},
		{ID: 7, Type: "remove", ChatRoomID: "hub-id", UserID: "user2", SessionID: "session2"},
		{ID: 8, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: "user1 left"},
		{ID: 9, Type: "close", ChatRoomID: "hub-id"},
	}

	expectedEvents22 := []Event{
		{ID: 3, Type: "add", ChatRoomID: "hub-id", UserID: "user2", SessionID: "session2"},
		{ID: 4, Type: "join", ChatRoomID: "hub-id", UserID: "user3", SessionID: "session1"},
		{ID: 5, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: "hello"},
		{ID: 6, Type: "leave", ChatRoomID: "hub-id", UserID: "user1", SessionID: "session1"},
	}

	expectedEvents3 := []Event{
		{ID: 4, Type: "join", ChatRoomID: "hub-id", UserID: "user3", SessionID: "session1"},
		{ID: 5, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: "hello"},
		{ID: 6, Type: "leave", ChatRoomID: "hub-id", UserID: "user1", SessionID: "session1"},
		{ID: 7, Type: "remove", ChatRoomID: "hub-id", UserID: "user2", SessionID: "session2"},
		{ID: 8, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: "user1 left"},
		{ID: 9, Type: "close", ChatRoomID: "hub-id"},
	}

	expectedHistory := []Event{
		{ID: 1, Type: "join", ChatRoomID: "hub-id", UserID: "user1", SessionID: "session1"},
		{ID: 2, Type: "join", ChatRoomID: "hub-id", UserID: "user2", SessionID: "session1"},
		{ID: 3, Type: "add", ChatRoomID: "hub-id", UserID: "user2", SessionID: "session2"},
		{ID: 4, Type: "join", ChatRoomID: "hub-id", UserID: "user3", SessionID: "session1"},
		{ID: 5, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: "hello"},
		{ID: 6, Type: "leave", ChatRoomID: "hub-id", UserID: "user1", SessionID: "session1"},
		{ID: 7, Type: "remove", ChatRoomID: "hub-id", UserID: "user2", SessionID: "session2"},
		{ID: 8, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: "user1 left"},
		{ID: 9, Type: "close", ChatRoomID: "hub-id"},
	}

	assert.Equal(t, expectedEvents1, removeTime(user1ReceivedEvents), "user1 should have received the correct events")
	assert.Equal(t, expectedEvents21, removeTime(user2ReceivedEvents1), "user2 session1 should have received the correct events")
	assert.Equal(t, expectedEvents22, removeTime(user2ReceivedEvents2), "user2 session2 should have received the correct events")
	assert.Equal(t, expectedEvents3, removeTime(user3ReceivedEvents), "user3 should have received the correct events")
	assert.Equal(t, expectedHistory, removeTime(hub.History()), "history should contain all the events")

	// .Message() should be no-op on closed MessageHUb
	hub.Message("send-id", "hello")

	assert.Equal(t, 0, len(user1Events), "user1 should have 0 events in events buffer")
	assert.Equal(t, 0, len(user2Events1), "user2 session1 should have 0 events in events buffer")
	assert.Equal(t, 0, len(user2Events2), "user2 session2 should have 0 events in events buffer")
	assert.Equal(t, 0, len(user3Events), "user3 should have 0 events in events buffer")

	// Operating on closed hub should return ErrClosed.
	_, err = hub.Add("user4", "session1", 10)
	if err != ErrClosed {
		t.Fatal("should have returned error")
	}
	err = hub.Remove("user2", "session1")
	if err != ErrClosed {
		t.Fatal("should have returned error")
	}
	err = hub.Close()
	if err != ErrClosed {
		t.Fatal("should have returned error")
	}

}

func TestMessageHub_AttachListener(t *testing.T) {

	wg := sync.WaitGroup{}
	wg.Add(2)

	// Set up middleware which intercepts and stores the events.
	listenerEvents := make([]Event, 0)

	listener := make(chan Event, 100)
	go func() {
		defer wg.Done()
		for event := range listener {
			listenerEvents = append(listenerEvents, event)
		}
	}()

	hub := New("hub-id", 10, 10)
	hub.AttachListener(listener)

	userEvents, err := hub.Add("user1", "session1", 10)
	if err != nil {
		t.Fatal(err)
	}

	userReceivedEvents := make([]Event, 0)

	go func() {
		defer wg.Done()
		for event := range userEvents {
			userReceivedEvents = append(userReceivedEvents, event)
		}
	}()

	hub.Message("send-id", "message1")
	hub.Message("send-id", "message2")
	hub.Message("send-id", "message3")
	hub.Close()

	wg.Wait()

	assert.Equal(t, listenerEvents, userReceivedEvents, "middleware events should be equal to user received events")

}

// removeTime removes the timestamp for comparison as this will be non-deterministic.
func removeTime(events []Event) []Event {
	modEvents := make([]Event, len(events))
	for i := range events {
		event := events[i]
		event.UnixTime = 0
		modEvents[i] = event
	}
	return modEvents
}
