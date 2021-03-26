package messagehub

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChatRoom(t *testing.T) {
	chatRoom := NewChatRoom("chatroomid", 10)

	user1Events, pastEvents1, err := chatRoom.Join("user1", 10)
	if err != nil {
		t.Fatal(err)
	}
	user2Events, pastEvents2, err := chatRoom.Join("user2", 10)
	if err != nil {
		t.Fatal(err)
	}
	user3Events, pastEvents3, err := chatRoom.Join("user3", 10)
	if err != nil {
		t.Fatal(err)
	}

	expectedPastEvents1 := []Event{
		{ID: 1, Type: "join", UserID: "user1"},
	}
	expectedPastEvents2 := []Event{
		{ID: 1, Type: "join", UserID: "user1"},
		{ID: 2, Type: "join", UserID: "user2"},
	}
	expectedPastEvents3 := []Event{
		{ID: 1, Type: "join", UserID: "user1"},
		{ID: 2, Type: "join", UserID: "user2"},
		{ID: 3, Type: "join", UserID: "user3"},
	}

	assert.Equal(t, expectedPastEvents1, removeTime(pastEvents1), "user1 should get the correct past events when joining")
	assert.Equal(t, expectedPastEvents2, removeTime(pastEvents2), "user2 should get the correct past events when joining")
	assert.Equal(t, expectedPastEvents3, removeTime(pastEvents3), "user3 should get the correct past events when joining")

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

	chatRoom.Message("send-id", []byte("hello"))
	chatRoom.Leave("user1")
	chatRoom.Message("send-id", []byte("user1 left"))
	chatRoom.Close()

	wg.Wait()

	expectedEvents1 := []Event{
		{ID: 1, Type: "join", UserID: "user1"},
		{ID: 2, Type: "join", UserID: "user2"},
		{ID: 3, Type: "join", UserID: "user3"},
		{ID: 4, Type: "message", UserID: "send-id", Message: []byte("hello")},
	}

	expectedEvents2 := []Event{
		{ID: 2, Type: "join", UserID: "user2"},
		{ID: 3, Type: "join", UserID: "user3"},
		{ID: 4, Type: "message", UserID: "send-id", Message: []byte("hello")},
		{ID: 5, Type: "leave", UserID: "user1"},
		{ID: 6, Type: "message", UserID: "send-id", Message: []byte("user1 left")},
		{ID: 7, Type: "close"},
	}

	expectedEvents3 := []Event{
		{ID: 3, Type: "join", UserID: "user3"},
		{ID: 4, Type: "message", UserID: "send-id", Message: []byte("hello")},
		{ID: 5, Type: "leave", UserID: "user1"},
		{ID: 6, Type: "message", UserID: "send-id", Message: []byte("user1 left")},
		{ID: 7, Type: "close"},
	}

	assert.Equal(t, expectedEvents1, removeTime(user1ReceivedEvents), "user1 should have received the correct events")
	assert.Equal(t, expectedEvents2, removeTime(user2ReceivedEvents), "user2 should have received the correct events")
	assert.Equal(t, expectedEvents3, removeTime(user3ReceivedEvents), "user3 should have received the correct events")

	// .Message() should be no-op on closed ChatRoom
	chatRoom.Message("send-id", []byte("hello"))

	assert.Equal(t, 0, len(user1Events), "user1 should have 0 events in events buffer")
	assert.Equal(t, 0, len(user2Events), "user2 should have 0 events in events buffer")
	assert.Equal(t, 0, len(user3Events), "user3 should have 0 events in events buffer")

	// Operating on closed chatroom should return ErrClosed.
	_, _, err = chatRoom.Join("user4", 10)
	if err != ErrClosed {
		t.Fatal("should have returned error")
	}
	err = chatRoom.Leave("user2")
	if err != ErrClosed {
		t.Fatal("should have returned error")
	}
	err = chatRoom.Close()
	if err != ErrClosed {
		t.Fatal("should have returned error")
	}

}

func TestChatRoom_Middleware(t *testing.T) {

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

	chatRoom := NewChatRoom("chatroomid", 10, storageMiddleware)

	userEvents, _, err := chatRoom.Join("user1", 10)
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

	chatRoom.Message("send-id", []byte("message1"))
	chatRoom.Message("send-id", []byte("message2"))
	chatRoom.Message("send-id", []byte("message3"))
	chatRoom.Close()

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
