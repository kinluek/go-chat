package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/kinluek/go-chat/backend/messagehub"
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

// ChatSocket handles a websocket connection that interacts with the message hub.
func ChatSocket(hub *messagehub.MessageHub, upgrader *websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[INFO]: connecting new user")

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[ERROR]: failed to upgrade connection - %s", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		username := r.FormValue("username")
		if username == "" {
			log.Printf("[INFO]: request missing username")
			return
		}
		sessionID := r.FormValue("sessionid")
		if sessionID == "" {
			log.Printf("[INFO]: request missing sessionid")
			return
		}
		eventStream, err := hub.Add(username, sessionID, 100)
		if err != nil {
			log.Printf("[ERROR]: failed to join - %v", err)
			return
		}
		defer hub.Remove(username, sessionID)

		done := make(chan struct{}, 2)
		go readMessages(username, conn, hub, done)
		go sendMessages(username, conn, eventStream, done)
		<-done

		log.Printf("[INFO]: user %s disconnected", username)
	}
}

type eventHistory struct {
	Events []messagehub.Event `json:"events"`
}

// ChatHistory returns the in-memory events for message hub.
func ChatHistory(hub *messagehub.MessageHub) http.HandlerFunc {
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
