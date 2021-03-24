package messagehub

import (
	"sync"

	"github.com/pkg/errors"
)

// MessageHub holds the available ChatRoom and provides thread safe access to them.
type MessageHub struct {
	chatRooms map[string]*ChatRoom

	mtx sync.RWMutex
}

// New returns an new instance of the MessageHub
func New() *MessageHub {
	return &MessageHub{chatRooms: make(map[string]*ChatRoom)}
}

// NewChatRoom creates a new chatroom.
func (hub *MessageHub) NewChatRoom(id string, bufferSize int) (*ChatRoom, error) {
	hub.mtx.Lock()
	defer hub.mtx.Unlock()
	_, ok := hub.chatRooms[id]
	if ok {
		return nil, errors.Errorf("chatroom %s already exists", id)
	}
	chatRoom := NewChatRoom(id, bufferSize)
	hub.chatRooms[id] = chatRoom
	return chatRoom, nil
}

// GetChatRoom returns returns the ChatRoom with the matching id.
func (hub *MessageHub) GetChatRoom(id string) (*ChatRoom, error) {
	hub.mtx.RLock()
	defer hub.mtx.RUnlock()
	chatRoom, ok := hub.chatRooms[id]
	if !ok {
		return nil, errors.Errorf("chatroom %s does not exist", id)
	}
	return chatRoom, nil
}
