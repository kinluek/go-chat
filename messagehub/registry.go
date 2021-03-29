package messagehub

import (
	"sync"

	"github.com/pkg/errors"
)

// Registry holds the available ChatRoom and provides thread safe access to them.
type Registry struct {
	hubs map[string]*MessageHub

	mtx sync.RWMutex
}

// NewRegistry returns an new instance of the MessageHub
func NewRegistry() *Registry {
	return &Registry{hubs: make(map[string]*MessageHub)}
}

// NewMessageHub creates a new MessageHub.
func (hub *Registry) NewMessageHub(id string, reqBufferSize, historySize int) (*MessageHub, error) {
	hub.mtx.Lock()
	defer hub.mtx.Unlock()
	_, ok := hub.hubs[id]
	if ok {
		return nil, errors.Errorf("messagehub %s already exists", id)
	}
	chatRoom := New(id, reqBufferSize, historySize)
	hub.hubs[id] = chatRoom
	return chatRoom, nil
}

// GetMessageHub returns returns the MessageHub with the matching id.
func (hub *Registry) GetMessageHub(id string) (*MessageHub, error) {
	hub.mtx.RLock()
	defer hub.mtx.RUnlock()
	chatRoom, ok := hub.hubs[id]
	if !ok {
		return nil, errors.Errorf("messagehub %s does not exist", id)
	}
	return chatRoom, nil
}
