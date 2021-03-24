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

	user1Events, pastEvents1 := chatRoom.Join("user1", 10)
	user2Events, pastEvents2 := chatRoom.Join("user2", 10)
	user3Events, pastEvents3 := chatRoom.Join("user3", 10)

	expectedPastEvents1 := []Event{
		{ID: 1, Type: "join", UserID: "user1", Message: nil},
	}
	expectedPastEvents2 := []Event{
		{ID: 1, Type: "join", UserID: "user1", Message: nil},
		{ID: 2, Type: "join", UserID: "user2", Message: nil},
	}
	expectedPastEvents3 := []Event{
		{ID: 1, Type: "join", UserID: "user1", Message: nil},
		{ID: 2, Type: "join", UserID: "user2", Message: nil},
		{ID: 3, Type: "join", UserID: "user3", Message: nil},
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
		defer wg.Done()
		for event := range user1Events {
			user1ReceivedEvents = append(user1ReceivedEvents, event)
			if event.Type == EventTypeMessage {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for event := range user2Events {
			user2ReceivedEvents = append(user2ReceivedEvents, event)
			if event.Type == EventTypeMessage {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for event := range user3Events {
			user3ReceivedEvents = append(user3ReceivedEvents, event)
			if event.Type == EventTypeMessage {
				return
			}
		}
	}()

	chatRoom.Message("send-id", []byte("hello"))

	wg.Wait()

	expectedEvents1 := []Event{
		{ID: 1, Type: "join", UserID: "user1", Message: nil},
		{ID: 2, Type: "join", UserID: "user2", Message: nil},
		{ID: 3, Type: "join", UserID: "user3", Message: nil},
		{ID: 4, Type: "message", UserID: "send-id", Message: []byte("hello")}}

	expectedEvents2 := []Event{
		{ID: 2, Type: "join", UserID: "user2", Message: nil},
		{ID: 3, Type: "join", UserID: "user3", Message: nil},
		{ID: 4, Type: "message", UserID: "send-id", Message: []byte("hello")}}

	expectedEvents3 := []Event{
		{ID: 3, Type: "join", UserID: "user3", Message: nil},
		{ID: 4, Type: "message", UserID: "send-id", Message: []byte("hello")},
	}

	assert.Equal(t, expectedEvents1, removeTime(user1ReceivedEvents), "user1 should have received the correct events")
	assert.Equal(t, expectedEvents2, removeTime(user2ReceivedEvents), "user2 should have received the correct events")
	assert.Equal(t, expectedEvents3, removeTime(user3ReceivedEvents), "user3 should have received the correct events")

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
			events, _ := chatRoom.Join("userid", tt.eventCount)

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
