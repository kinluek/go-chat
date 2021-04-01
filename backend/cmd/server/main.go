package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kinluek/go-chat/backend/cmd/server/handlers"
	"github.com/kinluek/go-chat/backend/datalog"
	"github.com/kinluek/go-chat/backend/messagehub"
)

var (
	port              int
	socketBufferSize  int
	hubBufferSize     int
	eventCacheSize    int
	datalogBufferSize int
	eventLog          string
	flushInterval     int
)

func main() {
	flag.IntVar(&port, "port", 8080, "port for server to listen on")
	flag.IntVar(&socketBufferSize, "socket-buffer-size", 1024, "sets the size of the read and write socket buffers")
	flag.IntVar(&hubBufferSize, "hub-buffer-size", 1024, "how many messages the message hub can queue up asynchronously")
	flag.IntVar(&eventCacheSize, "event-cache-size", 1024, "how many events the message hub will cache in memory")
	flag.IntVar(&datalogBufferSize, "datalog-buffer-size", 1024, "size of the datalog channel buffer")
	flag.StringVar(&eventLog, "event-log-path", "./events.log", "the file path to log events for persistence")
	flag.IntVar(&flushInterval, "flush-interval-secs", 5, "how often to flush events to log file in seconds")
	flag.Parse()
	flushSecs := time.Duration(flushInterval) * time.Second

	log.Printf("config - port=%v socket-buffer-size=%v hub-buffer-size=%v event-cache-size=%v event-log-path=%q flush-interval-secs=%v",
		port, socketBufferSize, hubBufferSize, eventCacheSize, eventLog, flushInterval,
	)

	// Create log file to write message hub events to.
	eventLog, err := os.OpenFile(eventLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer eventLog.Close()

	wg := sync.WaitGroup{}

	// Setup the message hub.
	hub := messagehub.New("GoChat", hubBufferSize, eventCacheSize)
	hub.AttachListener(datalog.Listener(eventLog, flushSecs, datalogBufferSize, &wg))

	// Create upgrader which upgrades standard http connections into websocket connections.
	upgrader := newUpgrader(socketBufferSize)

	// Attach handlers and serve.
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", handlers.ChatSocket(hub, upgrader))
	mux.HandleFunc("/history", handlers.ChatHistory(hub))

	server := http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%v", port),
		Handler: mux,
	}
	go func() {
		log.Printf("starting server on port %v", port)
		err := server.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Listen for shutdown signal so we can do a simple cleanup before exiting.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	<-shutdown

	log.Println("shutting down...")
	hub.Close()
	wg.Wait()
	server.Close()

}

func newUpgrader(bufferSize int) *websocket.Upgrader {
	return &websocket.Upgrader{
		ReadBufferSize:  bufferSize,
		WriteBufferSize: bufferSize,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
}
