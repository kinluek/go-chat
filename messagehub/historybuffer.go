package messagehub

import "sync"

// eventHistoryBuffer holds the last n (size) events in memory. The buffer
// is updated in a circular fashion for performance and efficiency.
type eventHistoryBuffer struct {
	buffer []Event
	size   int
	head   int
	tail   int

	mtx sync.Mutex
}

func newEventsHistoryBuffer(size int) *eventHistoryBuffer {
	return &eventHistoryBuffer{
		buffer: make([]Event, 0, size),
		size:   size,
		head:   -1,
	}
}

func (b *eventHistoryBuffer) push(event Event) {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	if len(b.buffer) < b.size {
		// if we have not reached capacity then we append
		b.buffer = append(b.buffer, event)
		b.head++
	} else {
		// start updating in circular fashion.
		b.head++
		if b.head == b.size {
			b.head = 0
		}
		b.tail++
		if b.tail == b.size {
			b.tail = 0
		}
		b.buffer[b.head] = event
	}
}

// events returns the buffered events in the order they were pushed in, from tail to head.
func (b *eventHistoryBuffer) events() []Event {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if len(b.buffer) == 0 {
		return nil
	}

	sortedEvents := make([]Event, b.size)
	i := 0
	tail := b.tail
	for {
		sortedEvents[i] = b.buffer[tail]
		if tail == b.head {
			break
		}
		i++
		tail++
		if tail == b.size {
			tail = 0
		}
	}
	return sortedEvents[:i+1]
}
