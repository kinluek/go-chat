package messagehub

import "testing"

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	id1 := "1"
	id2 := "2"

	newHub1, err := registry.NewMessageHub(id1, 100)
	if err != nil {
		t.Fatalf("failed to create initial message hub id[%s]: %s", id1, err)
	}

	gotHub1, err := registry.GetMessageHub(id1)
	if newHub1 != gotHub1 {
		t.Fatalf("should get the same message hub for same id - id[%s]", id1)
	}

	newHub2, err := registry.NewMessageHub(id2, 100)
	if err != nil {
		t.Fatalf("failed to create initial message hub id[%s]: %s", id2, err)
	}

	gotHub2, err := registry.GetMessageHub(id2)
	if newHub2 != gotHub2 {
		t.Fatalf("should get the same message hub for same id - id[%s]", id2)
	}

	if newHub1 == newHub2 {
		t.Fatalf("new hub should not references other message hub")
	}

	_, err = registry.NewMessageHub(id1, 100)
	if err == nil {
		t.Fatalf("should return an error for creating a message hub with same id")
	}

	_, err = registry.GetMessageHub("invalid")
	if err == nil {
		t.Fatalf("should return an error for getting non existant message hub")
	}
}
