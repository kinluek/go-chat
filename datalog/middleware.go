package datalog

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/kinluek/go-chat/messagehub"
)

// Store creates messagehub.MiddleWare that can be used to log the events to a file for persistence.
// A buffered writer is used to speed up writes, the writer is flushed on the flushInterval.
func Store(logFile *os.File, flushInterval time.Duration) messagehub.MiddleWare {
	mid := func(in <-chan messagehub.Request) <-chan messagehub.Request {
		out := make(chan messagehub.Request)
		go func() {
			ticker := time.NewTicker(flushInterval)
			writer := bufio.NewWriter(logFile)
			encoder := json.NewEncoder(writer)
			defer func() {
				ticker.Stop()
				writer.Flush()
				close(out)
			}()
			for {
				select {
				case req, ok := <-in:
					if !ok {
						return
					}
					if err := encoder.Encode(req.Event); err != nil {
						log.Printf("[ERROR]: failed to write event - %#v - %v", req.Event, err)
					}
					out <- req
				case <-ticker.C:
					if err := writer.Flush(); err != nil {
						log.Printf("[ERROR]: failed to flush event logs - %#v", err)
					}
				}
			}
		}()
		return out
	}
	return mid
}
