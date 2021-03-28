package datalog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/kinluek/go-chat/messagehub"
	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	fileName := "test.log"
	logFile, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(fileName)

	// create input stream
	input := make(chan messagehub.Request)

	// wrap input stream with store middleware
	out := Store(logFile, time.Millisecond)(input)

	wg := sync.WaitGroup{}
	wg.Add(1)

	// drain output stream so requests are pulled through
	go func() {
		// out should be closed after input is closed and writer is flushed.
		for range out {
		}
		wg.Done()
	}()

	inputs := []messagehub.Request{
		{
			Event: messagehub.Event{ID: 1},
		},
		{
			Event: messagehub.Event{ID: 2},
		},
		{
			Event: messagehub.Event{ID: 3},
		},
		{
			Event: messagehub.Event{ID: 4},
		},
	}

	for _, req := range inputs {
		input <- req
	}
	close(input)
	wg.Wait()

	got := readEventsFromFile(t, logFile)
	want := []messagehub.Event{
		{ID: 1},
		{ID: 2},
		{ID: 3},
		{ID: 4},
	}
	assert.Equal(t, want, got, "logged event should match inputs")
}

func readEventsFromFile(t *testing.T, logFile *os.File) []messagehub.Event {
	t.Helper()
	logFile.Seek(0, 0)
	events := make([]messagehub.Event, 0)
	scanner := bufio.NewScanner(logFile)
	for scanner.Scan() {
		var event messagehub.Event
		err := json.Unmarshal(bytes.TrimSpace(scanner.Bytes()), &event)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	return events
}
