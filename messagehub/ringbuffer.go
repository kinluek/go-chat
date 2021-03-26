package messagehub

// eventsRingBuffer holds the last n (size) events in memory
// the buffer is updated in a circular fashion for performance.
// (Instead of popping and pushing to a slice, which would require
// a lot of memory allocations)
type eventsRingBuffer struct {
	buffer []Event
	size   int
	head   int
	tail   int

	notEmpty bool
	wrapped  bool
}

func newEventsRingBuffer(size int) *eventsRingBuffer {
	return &eventsRingBuffer{
		buffer: make([]Event, size),
		size:   size,
	}
}

func (b *eventsRingBuffer) push(event Event) {
	if b.notEmpty {
		b.head++
		if b.head == b.size {
			b.head = 0
			b.wrapped = true
		}
		if b.wrapped {
			b.tail++
		}
		if b.tail == b.size {
			b.tail = 0
		}
	}
	b.buffer[b.head] = event

	if !b.notEmpty {
		b.notEmpty = true
	}
}

func (b *eventsRingBuffer) makeCopy() *eventsRingBuffer {
	copiedBuf := make([]Event, b.size)
	copy(copiedBuf, b.buffer)

	return &eventsRingBuffer{
		buffer:   copiedBuf,
		size:     b.size,
		head:     b.head,
		tail:     b.tail,
		notEmpty: b.notEmpty,
		wrapped:  b.wrapped,
	}
}

func (b *eventsRingBuffer) events() []Event {
	if !b.notEmpty || len(b.buffer) == 0 {
		return nil
	}
	sorted := make([]Event, b.size)

	i := 0
	tail := b.tail
	for {
		sorted[i] = b.buffer[tail]
		if tail == b.head {
			break
		}
		i++
		tail++
		if tail == b.size {
			tail = 0
		}
	}
	return sorted[:i+1]
}
