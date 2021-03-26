package messagehub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_eventsRingBuffer_push(t *testing.T) {
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
				{ID: 1},
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
				{ID: 1},
				{ID: 2},
				{ID: 3},
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
				{ID: 1},
				{ID: 2},
				{ID: 3},
				{ID: 4},
				{ID: 5},
			},
		},
		{
			name:       "events-count-1-over-the-buffer-size",
			bufferSize: 5,
			eventCount: 6,
			wantHead:   0,
			wantTail:   1,
			wantBuffer: []Event{
				{ID: 6},
				{ID: 2},
				{ID: 3},
				{ID: 4},
				{ID: 5},
			},
		},
		{
			name:       "events-count-2-over-the-buffer-size",
			bufferSize: 5,
			eventCount: 7,
			wantHead:   1,
			wantTail:   2,
			wantBuffer: []Event{
				{ID: 6},
				{ID: 7},
				{ID: 3},
				{ID: 4},
				{ID: 5},
			},
		},
		{
			name:       "events-count-2-over-2-times-the-buffer-size",
			bufferSize: 5,
			eventCount: 12,
			wantHead:   1,
			wantTail:   2,
			wantBuffer: []Event{
				{ID: 11},
				{ID: 12},
				{ID: 8},
				{ID: 9},
				{ID: 10},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ringBuffer := newEventsRingBuffer(tt.bufferSize)
			// we start from 1 as the Join counts as event.
			for i := 1; i <= tt.eventCount; i++ {
				ringBuffer.push(Event{ID: i})
			}

			assert.Equal(t, tt.wantHead, ringBuffer.head, "head should equal")
			assert.Equal(t, tt.wantTail, ringBuffer.tail, "tail should equal")
			assert.Equal(t, tt.wantBuffer, ringBuffer.buffer, "events in buffer should equal")
		})
	}
}

func Test_eventsRingBuffer_events(t *testing.T) {
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
			wantEvents:  nil,
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
			ringBuffer := eventsRingBuffer{
				buffer:   tt.inputEvents,
				head:     tt.head,
				tail:     tt.tail,
				size:     len(tt.inputEvents),
				notEmpty: true,
			}
			got := ringBuffer.events()
			assert.Equal(t, tt.wantEvents, got, "returned events should be sorted and trimmed")
		})
	}
}
