package datalog

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/kinluek/go-chat/messagehub"
)

// Listener creates messagehub.Listener that can be used to log the events to a file for persistence.
// A buffered writer is used to speed up writes, the writer is flushed on the flushInterval.
// A WaitGroup can be used to wait on to make sure the store is completely flushed before shutting down.
func Listener(logFile *os.File, flushInterval time.Duration, buffSize int, wg *sync.WaitGroup) chan<- messagehub.Event {
	listener := make(chan messagehub.Event, buffSize)
	wg.Add(1)
	go func() {
		ticker := time.NewTicker(flushInterval)
		writer := bufio.NewWriter(logFile)
		encoder := json.NewEncoder(writer)
		defer func() {
			ticker.Stop()
			writer.Flush()
			wg.Done()
		}()
		for {
			select {
			case event, ok := <-listener:
				if !ok {
					return
				}
				if err := encoder.Encode(event); err != nil {
					log.Printf("[ERROR]: failed to write event - %#v - %v", event, err)
				}
			case <-ticker.C:
				if err := writer.Flush(); err != nil {
					log.Printf("[ERROR]: failed to flush event logs - %#v", err)
				}
			}
		}
	}()
	return listener
}
