package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kinluek/go-chat/datalog"
	"github.com/kinluek/go-chat/messagehub"
)

// readMessages will be run for each connection, it pumps messages from the client the message hub.
func readMessages(userID string, conn *websocket.Conn, hub *messagehub.MessageHub, done chan struct{}) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("[ERROR]: unexpected close error - %v", err)
			}
			done <- struct{}{}
			return
		}
		text := strings.TrimSpace(string(message))
		hub.Message(userID, text)
	}
}

// sendMessages will be run for each connection, it pumps events from the message hub to the client.
func sendMessages(userID string, conn *websocket.Conn, events <-chan messagehub.Event, done chan struct{}) {
	for event := range events {
		if err := conn.WriteJSON(event); err != nil {
			log.Printf("[ERROR]: failed to write message for event id %v - %v", event.ID, err)
			done <- struct{}{}
			return
		}
	}
	// hub has closed the events stream
	done <- struct{}{}
	return
}

// handleConnection handles a client websocket connection.
func handleConnection(hub *messagehub.MessageHub, upgrader *websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[INFO]: connecting new user")

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[ERROR]: failed to upgrade connection - %s", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		userName := r.FormValue("username")
		if userName == "" {
			log.Printf("[INFO]: request missing username")
			return
		}
		eventStream, err := hub.Join(userName, 100)
		if err != nil {
			log.Printf("[ERROR]: failed to join - %v", err)
			return
		}
		defer hub.Leave(userName)

		done := make(chan struct{}, 2)
		go readMessages(userName, conn, hub, done)
		go sendMessages(userName, conn, eventStream, done)
		<-done

		log.Printf("[INFO]: user %s disconnected", userName)
	}
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

type eventHistory struct {
	Events []messagehub.Event `json:"events"`
}

// getEventsHistory returns the in-memory events for message hub.
func getEventsHistory(hub *messagehub.MessageHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[INFO]: getting history")
		history := hub.History()
		resp, err := json.Marshal(eventHistory{history})
		if err != nil {
			log.Printf("[ERROR]: failed to marshal event history - %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(resp)
		if err != nil {
			log.Printf("[ERROR]: failed to write history response - %v", err)
		}
	}
}

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

	logFile, err := os.OpenFile(eventLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	wg := sync.WaitGroup{}
	hub := messagehub.New("GoChat", hubBufferSize, eventCacheSize)
	hub.AttachListener(datalog.Listener(logFile, flushSecs, datalogBufferSize, &wg))

	upgrader := newUpgrader(socketBufferSize)

	http.HandleFunc("/chat", handleConnection(hub, upgrader))
	http.HandleFunc("/history", getEventsHistory(hub))

	go func() {
		log.Printf("starting server on port %v", port)
		err = http.ListenAndServe(":"+strconv.Itoa(port), nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// listen to shutdown signal so we can do a simple cleanup before exiting.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	<-shutdown

	log.Println("shutting down...")

	hub.Close()
	wg.Wait()

	log.Println("shutdown complete")
}
