package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/kinluek/go-chat/messagehub"
	"github.com/pkg/errors"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

func readMessages(userID string, conn *websocket.Conn, hub *messagehub.MessageHub, errs chan<- error) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			errs <- errors.Wrap(err, "failed to read message")
			return
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		hub.Message(userID, message)
	}
}

func sendMessages(userID string, conn *websocket.Conn, events <-chan messagehub.Event, errs chan<- error) {
	for event := range events {
		w, err := conn.NextWriter(websocket.TextMessage)
		if err != nil {
			errs <- errors.Wrap(err, "failed to get next writer")
			return
		}
		msg, err := json.Marshal(event)
		if err != nil {
			errs <- errors.Wrap(err, "failed to marshal event")
			return
		}
		w.Write(msg)
		if err := w.Close(); err != nil {
			errs <- errors.Wrap(err, "failed to close writer")
			return
		}
	}
}

func handleConnection(hub *messagehub.MessageHub) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
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

		errs := make(chan error, 2)

		eventStream, _, err := hub.Join(userName, 100)
		if err != nil {
			log.Printf("[ERROR]: failed to join - %v", err)
			return
		}
		defer hub.Leave(userName)

		go readMessages(userName, conn, hub, errs)
		go sendMessages(userName, conn, eventStream, errs)

		for {
			select {
			case err := <-errs:
				log.Printf("[ERROR]: %v", err)
				return
			case <-r.Context().Done():
				return
			}
		}
	}
	return handler
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "public/index.html")
}

var (
	port            int
	eventBufferSize int
)

func main() {
	flag.IntVar(&port, "port", 8080, "port for server to listen on")
	flag.IntVar(&eventBufferSize, "event-buffer-size", 1024, "number of events the message hub can hold in memory")

	hub := messagehub.New("my-chat", eventBufferSize)

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", handleConnection(hub))

	if err := http.ListenAndServe(":"+strconv.Itoa(port), nil); err != nil {
		log.Fatal(err)
	}
}
