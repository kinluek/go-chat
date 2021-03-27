package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/kinluek/go-chat/messagehub"
)

// hub is the MessageHub that we set globally for simplicity.
var hub = messagehub.New("GoChat", 1024)

// upgrader upgrades a standard http request to a websocket connection.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

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
}

// handleConnection handles a client websocket connection.
func handleConnection(w http.ResponseWriter, r *http.Request) {
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

type eventHistory struct {
	Events []messagehub.Event `json:"events"`
}

// getEventsHistory returns the in-memory events for message hub.
func getEventsHistory(w http.ResponseWriter, r *http.Request) {
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
	w.Write(resp)
}

var (
	port int
)

func main() {
	flag.IntVar(&port, "port", 8080, "port for server to listen on")
	flag.Parse()

	http.HandleFunc("/chat", handleConnection)
	http.HandleFunc("/history", getEventsHistory)

	log.Printf("starting server on port %v", port)
	err := http.ListenAndServe(":"+strconv.Itoa(port), nil)
	if err != nil {
		log.Fatal(err)
	}
}
