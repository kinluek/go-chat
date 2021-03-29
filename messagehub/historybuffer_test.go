package messagehub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_eventHistoryBuffer(t *testing.T) {
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
			wantBuffer: []Event{
				{ID: 1},
			},
		},
		{
			name:       "3-event",
			bufferSize: 5,
			eventCount: 3,
			wantBuffer: []Event{
				{ID: 1},
				{ID: 2},
				{ID: 3},
			},
		},
		{
			name:       "events-count-equal-to-buffer-size",
			bufferSize: 5,
			eventCount: 5,
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
			wantBuffer: []Event{
				{ID: 2},
				{ID: 3},
				{ID: 4},
				{ID: 5},
				{ID: 6},
			},
		},
		{
			name:       "events-count-2-over-the-buffer-size",
			bufferSize: 5,
			eventCount: 7,
			wantBuffer: []Event{
				{ID: 3},
				{ID: 4},
				{ID: 5},
				{ID: 6},
				{ID: 7},
			},
		},
		{
			name:       "events-count-2-over-2-times-the-buffer-size",
			bufferSize: 5,
			eventCount: 12,
			wantBuffer: []Event{
				{ID: 8},
				{ID: 9},
				{ID: 10},
				{ID: 11},
				{ID: 12},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ringBuffer := newEventsHistoryBuffer(tt.bufferSize)

			for i := 1; i <= tt.eventCount; i++ {
				ringBuffer.push(Event{ID: i})
			}

			assert.Equal(t, tt.wantBuffer, ringBuffer.events(), "events in buffer should equal")
		})
	}
}

// goos: darwin
// goarch: amd64
// pkg: github.com/kinluek/go-chat/messagehub
// cpu: Intel(R) Core(TM) i7-4770HQ CPU @ 2.20GHz
// Benchmark_eventsHistoryBuffer_push_1-8          160769388               31.16 ns/op            0 B/op          0 allocs/op
func Benchmark_eventsHistoryBuffer_push_1(b *testing.B) {
	buffer := newEventsHistoryBuffer(1)
	for n := 0; n < b.N; n++ {
		buffer.push(Event{})
	}
}

// goos: darwin
// goarch: amd64
// pkg: github.com/kinluek/go-chat/messagehub
// cpu: Intel(R) Core(TM) i7-4770HQ CPU @ 2.20GHz
// Benchmark_eventsHistoryBuffer_push_100-8        183115000               34.03 ns/op            0 B/op          0 allocs/op
func Benchmark_eventsHistoryBuffer_push_100(b *testing.B) {
	buffer := newEventsHistoryBuffer(100)
	for n := 0; n < b.N; n++ {
		buffer.push(Event{})
	}
}

// goos: darwin
// goarch: amd64
// pkg: github.com/kinluek/go-chat/messagehub
// cpu: Intel(R) Core(TM) i7-4770HQ CPU @ 2.20GHz
// Benchmark_eventsHistoryBuffer_push_1024-8       182881276               34.12 ns/op            0 B/op          0 allocs/op
func Benchmark_eventsHistoryBuffer_push_1024(b *testing.B) {
	buffer := newEventsHistoryBuffer(1024)
	for n := 0; n < b.N; n++ {
		buffer.push(Event{})
	}
}
