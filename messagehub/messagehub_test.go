package messagehub

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMessageHub(t *testing.T) {
	hub := New("hub-id", 10)

	user1Events, err := hub.Join("user1", 10)
	if err != nil {
		t.Fatal(err)
	}
	user2Events, err := hub.Join("user2", 10)
	if err != nil {
		t.Fatal(err)
	}
	user3Events, err := hub.Join("user3", 10)
	if err != nil {
		t.Fatal(err)
	}

	user1ReceivedEvents := make([]Event, 0)
	user2ReceivedEvents := make([]Event, 0)
	user3ReceivedEvents := make([]Event, 0)

	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		for event := range user1Events {
			user1ReceivedEvents = append(user1ReceivedEvents, event)
			if event.Type == EventTypeMessage {
				go func() {
					time.Sleep(20 * time.Millisecond)
					wg.Done()
				}()
			}
		}
	}()

	go func() {
		defer wg.Done()
		for event := range user2Events {
			user2ReceivedEvents = append(user2ReceivedEvents, event)
			if event.Type == EventTypeClose {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for event := range user3Events {
			user3ReceivedEvents = append(user3ReceivedEvents, event)
			if event.Type == EventTypeClose {
				return
			}
		}
	}()

	hub.Message("send-id", []byte("hello"))
	hub.Leave("user1")
	hub.Message("send-id", []byte("user1 left"))
	hub.Close()

	wg.Wait()

	expectedEvents1 := []Event{
		{ID: 1, Type: "join", ChatRoomID: "hub-id", UserID: "user1"},
		{ID: 2, Type: "join", ChatRoomID: "hub-id", UserID: "user2"},
		{ID: 3, Type: "join", ChatRoomID: "hub-id", UserID: "user3"},
		{ID: 4, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: []byte("hello")},
	}

	expectedEvents2 := []Event{
		{ID: 2, Type: "join", ChatRoomID: "hub-id", UserID: "user2"},
		{ID: 3, Type: "join", ChatRoomID: "hub-id", UserID: "user3"},
		{ID: 4, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: []byte("hello")},
		{ID: 5, Type: "leave", ChatRoomID: "hub-id", UserID: "user1"},
		{ID: 6, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: []byte("user1 left")},
		{ID: 7, Type: "close", ChatRoomID: "hub-id"},
	}

	expectedEvents3 := []Event{
		{ID: 3, Type: "join", ChatRoomID: "hub-id", UserID: "user3"},
		{ID: 4, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: []byte("hello")},
		{ID: 5, Type: "leave", ChatRoomID: "hub-id", UserID: "user1"},
		{ID: 6, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: []byte("user1 left")},
		{ID: 7, Type: "close", ChatRoomID: "hub-id"},
	}

	expectedHistory := []Event{
		{ID: 1, Type: "join", ChatRoomID: "hub-id", UserID: "user1"},
		{ID: 2, Type: "join", ChatRoomID: "hub-id", UserID: "user2"},
		{ID: 3, Type: "join", ChatRoomID: "hub-id", UserID: "user3"},
		{ID: 4, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: []byte("hello")},
		{ID: 5, Type: "leave", ChatRoomID: "hub-id", UserID: "user1"},
		{ID: 6, Type: "message", ChatRoomID: "hub-id", UserID: "send-id", Message: []byte("user1 left")},
		{ID: 7, Type: "close", ChatRoomID: "hub-id"},
	}

	assert.Equal(t, expectedEvents1, removeTime(user1ReceivedEvents), "user1 should have received the correct events")
	assert.Equal(t, expectedEvents2, removeTime(user2ReceivedEvents), "user2 should have received the correct events")
	assert.Equal(t, expectedEvents3, removeTime(user3ReceivedEvents), "user3 should have received the correct events")
	assert.Equal(t, expectedHistory, removeTime(hub.History()), "history should contain all the events")

	// .Message() should be no-op on closed MessageHUb
	hub.Message("send-id", []byte("hello"))

	assert.Equal(t, 0, len(user1Events), "user1 should have 0 events in events buffer")
	assert.Equal(t, 0, len(user2Events), "user2 should have 0 events in events buffer")
	assert.Equal(t, 0, len(user3Events), "user3 should have 0 events in events buffer")

	// Operating on closed hub should return ErrClosed.
	_, err = hub.Join("user4", 10)
	if err != ErrClosed {
		t.Fatal("should have returned error")
	}
	err = hub.Leave("user2")
	if err != ErrClosed {
		t.Fatal("should have returned error")
	}
	err = hub.Close()
	if err != ErrClosed {
		t.Fatal("should have returned error")
	}

}

func TestMessageHub_Middleware(t *testing.T) {

	wg := sync.WaitGroup{}
	wg.Add(2)

	// Set up middleware which intercepts and stores the events.
	storageEvents := make([]Event, 0)
	storageMiddleware := func(in <-chan Request) <-chan Request {
		out := make(chan Request)
		go func() {
			defer wg.Done()
			for req := range in {
				fmt.Printf("middleware event - %v\n", req.Event)
				storageEvents = append(storageEvents, req.Event)
				out <- req
			}
		}()
		return out
	}

	hub := New("hub-id", 10, storageMiddleware)

	userEvents, err := hub.Join("user1", 10)
	if err != nil {
		t.Fatal(err)
	}

	userReceivedEvents := make([]Event, 0)

	go func() {
		defer wg.Done()
		for event := range userEvents {
			fmt.Printf("user event - %v\n", event)
			userReceivedEvents = append(userReceivedEvents, event)
			if event.Type == EventTypeClose {
				return
			}
		}
	}()

	hub.Message("send-id", []byte("message1"))
	hub.Message("send-id", []byte("message2"))
	hub.Message("send-id", []byte("message3"))
	hub.Close()

	wg.Wait()

	assert.Equal(t, storageEvents, userReceivedEvents, "middleware events should be equal to user received events")

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
