package messagehub

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
)

func TestChatRoom(t *testing.T) {
	chatRoom := NewChatRoom()

	n := 4

	eventStreams := make([]chan Event, n)
	for i := 0; i < n; i++ {
		eventStreams[i] = make(chan Event, 100)
	}

	wg := sync.WaitGroup{}
	wg.Add(n)

	for i, eventStream := range eventStreams {
		go func(i int, eventStream <-chan Event) {
			defer wg.Done()
			for event := range eventStream {
				if event.Type == EventTypeMessage {
					fmt.Printf("eventstream[%v] - sendid[%s] - type[%s] - message[%s]\n", i, event.UserID, event.Type, event.Message)
					return
				}
			}
		}(i, eventStream)
	}

	for i := 0; i < n; i++ {
		chatRoom.Join(strconv.Itoa(i), eventStreams[i])
	}

	chatRoom.Message("cheese", []byte("hello"))
	wg.Wait()

}
