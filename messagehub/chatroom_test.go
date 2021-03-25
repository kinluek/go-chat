package messagehub

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChatRoom(t *testing.T) {
	chatRoom := NewChatRoom("chatroomid", 10)

	type user struct {
		id     string
		events chan Event
	}

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

func TestChatRoom_updateBuffer(t *testing.T) {
	// Test that the buffer updates in a circular fashion.

	tests := []struct {
		name       string
		bufferSize int
		eventCount int
		wantHead   int
		wantTail   int
		wantBuffer []Event
	}{
		{
			name:       "1-event",
			bufferSize: 5,
			eventCount: 1,
			wantHead:   0,
			wantTail:   0,
			wantBuffer: []Event{
				{ID: 1, Type: EventTypeJoin, UserID: "userid"},
				{},
				{},
				{},
				{},
			},
		},
		{
			name:       "3-event",
			bufferSize: 5,
			eventCount: 3,
			wantHead:   2,
			wantTail:   0,
			wantBuffer: []Event{
				{ID: 1, Type: EventTypeJoin, UserID: "userid"},
				{ID: 2, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 3, Type: EventTypeMessage, UserID: "senderid"},
				{},
				{},
			},
		},
		{
			name:       "events-count-equal-to-buffer-size",
			bufferSize: 5,
			eventCount: 5,
			wantHead:   4,
			wantTail:   0,
			wantBuffer: []Event{
				{ID: 1, Type: EventTypeJoin, UserID: "userid"},
				{ID: 2, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 3, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 4, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 5, Type: EventTypeMessage, UserID: "senderid"},
			},
		},
		{
			name:       "events-count-1-over-the-buffer-size",
			bufferSize: 5,
			eventCount: 6,
			wantHead:   0,
			wantTail:   1,
			wantBuffer: []Event{
				{ID: 6, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 2, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 3, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 4, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 5, Type: EventTypeMessage, UserID: "senderid"},
			},
		},
		{
			name:       "events-count-2-over-the-buffer-size",
			bufferSize: 5,
			eventCount: 7,
			wantHead:   1,
			wantTail:   2,
			wantBuffer: []Event{
				{ID: 6, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 7, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 3, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 4, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 5, Type: EventTypeMessage, UserID: "senderid"},
			},
		},
		{
			name:       "events-count-2-over-2-times-the-buffer-size",
			bufferSize: 5,
			eventCount: 12,
			wantHead:   1,
			wantTail:   2,
			wantBuffer: []Event{
				{ID: 11, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 12, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 8, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 9, Type: EventTypeMessage, UserID: "senderid"},
				{ID: 10, Type: EventTypeMessage, UserID: "senderid"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatRoom := NewChatRoom("id", tt.bufferSize)
			events, _, _ := chatRoom.Join("userid", tt.eventCount)

			errs := make(chan error, 1)

			wg := sync.WaitGroup{}
			wg.Add(1)

			go func() {
				timeout := time.NewTimer(time.Second)
				defer timeout.Stop()
				defer wg.Done()
				count := 0
				for {
					select {
					case <-events:
						count++
						if count == tt.eventCount {
							errs <- nil
							return
						}
						timeout.Reset(time.Second)
					case <-timeout.C:
						errs <- errors.New("blocked event stream")
						return
					}
				}
			}()

			// we start from 1 as the Join counts as event.
			for i := 1; i < tt.eventCount; i++ {
				chatRoom.Message("senderid", nil)
			}
			wg.Wait()

			if err := <-errs; err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, tt.wantHead, chatRoom.bufferHead, "head should equal")
			assert.Equal(t, tt.wantTail, chatRoom.bufferTail, "tail should equal")
			assert.Equal(t, tt.wantBuffer, removeTime(chatRoom.buffer), "events in buffer should equal")
		})
	}
}

func Test_sortEventsBuffer(t *testing.T) {
	tests := []struct {
		name        string
		head        int
		tail        int
		inputEvents []Event
		wantEvents  []Event
	}{
		{
			name:        "nil-events",
			head:        0,
			tail:        0,
			inputEvents: nil,
			wantEvents:  nil,
		},
		{
			name:        "empty-events",
			head:        0,
			tail:        0,
			inputEvents: []Event{},
			wantEvents:  []Event{},
		},
		{
			name: "1-event-needs-trimming",
			head: 0,
			tail: 0,
			inputEvents: []Event{
				{ID: 1, Type: EventTypeJoin, UserID: "userid"},
				{},
				{},
				{},
				{},
			},
			wantEvents: []Event{
				{ID: 1, Type: EventTypeJoin, UserID: "userid"},
			},
		},
		{
			name: "3-event-needs-trimming",
			head: 2,
			tail: 0,
			inputEvents: []Event{
				{ID: 1, Type: EventTypeJoin, UserID: "userid"},
				{ID: 2, Type: EventTypeJoin, UserID: "userid"},
				{ID: 3, Type: EventTypeJoin, UserID: "userid"},
				{},
				{},
			},
			wantEvents: []Event{
				{ID: 1, Type: EventTypeJoin, UserID: "userid"},
				{ID: 2, Type: EventTypeJoin, UserID: "userid"},
				{ID: 3, Type: EventTypeJoin, UserID: "userid"},
			},
		},
		{
			name: "full-buffer-already-ordered",
			head: 4,
			tail: 0,
			inputEvents: []Event{
				{ID: 1, Type: EventTypeJoin, UserID: "userid"},
				{ID: 2, Type: EventTypeJoin, UserID: "userid"},
				{ID: 3, Type: EventTypeJoin, UserID: "userid"},
				{ID: 4, Type: EventTypeJoin, UserID: "userid"},
				{ID: 5, Type: EventTypeJoin, UserID: "userid"},
			},
			wantEvents: []Event{
				{ID: 1, Type: EventTypeJoin, UserID: "userid"},
				{ID: 2, Type: EventTypeJoin, UserID: "userid"},
				{ID: 3, Type: EventTypeJoin, UserID: "userid"},
				{ID: 4, Type: EventTypeJoin, UserID: "userid"},
				{ID: 5, Type: EventTypeJoin, UserID: "userid"},
			},
		},
		{
			name: "wrapped-around",
			head: 0,
			tail: 1,
			inputEvents: []Event{
				{ID: 6, Type: EventTypeJoin, UserID: "userid"},
				{ID: 2, Type: EventTypeJoin, UserID: "userid"},
				{ID: 3, Type: EventTypeJoin, UserID: "userid"},
				{ID: 4, Type: EventTypeJoin, UserID: "userid"},
				{ID: 5, Type: EventTypeJoin, UserID: "userid"},
			},
			wantEvents: []Event{
				{ID: 2, Type: EventTypeJoin, UserID: "userid"},
				{ID: 3, Type: EventTypeJoin, UserID: "userid"},
				{ID: 4, Type: EventTypeJoin, UserID: "userid"},
				{ID: 5, Type: EventTypeJoin, UserID: "userid"},
				{ID: 6, Type: EventTypeJoin, UserID: "userid"},
			},
		},
		{
			name: "wrapped-around-twice",
			head: 1,
			tail: 2,
			inputEvents: []Event{
				{ID: 11, Type: EventTypeJoin, UserID: "userid"},
				{ID: 12, Type: EventTypeJoin, UserID: "userid"},
				{ID: 8, Type: EventTypeJoin, UserID: "userid"},
				{ID: 9, Type: EventTypeJoin, UserID: "userid"},
				{ID: 10, Type: EventTypeJoin, UserID: "userid"},
			},
			wantEvents: []Event{
				{ID: 8, Type: EventTypeJoin, UserID: "userid"},
				{ID: 9, Type: EventTypeJoin, UserID: "userid"},
				{ID: 10, Type: EventTypeJoin, UserID: "userid"},
				{ID: 11, Type: EventTypeJoin, UserID: "userid"},
				{ID: 12, Type: EventTypeJoin, UserID: "userid"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortEventsBuffer(tt.inputEvents, tt.head, tt.tail)
			assert.Equal(t, tt.wantEvents, got, "sorted events should match wanted")
		})
	}
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
