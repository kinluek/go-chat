package messagehub

import "testing"

func TestMessageHub(t *testing.T) {
	messageHub := New()

	id1 := "1"
	id2 := "2"

	newChatRoom1, err := messageHub.NewChatRoom(id1)
	if err != nil {
		t.Fatalf("failed to create initial chat room id[%s]: %s", id1, err)
	}

	gotChatRoom1, err := messageHub.GetChatRoom(id1)
	if newChatRoom1 != gotChatRoom1 {
		t.Fatalf("should get the same chatroom for same id - id[%s]", id1)
	}

	newChatRoom2, err := messageHub.NewChatRoom(id2)
	if err != nil {
		t.Fatalf("failed to create initial chat room id[%s]: %s", id2, err)
	}

	gotChatRoom2, err := messageHub.GetChatRoom(id2)
	if newChatRoom2 != gotChatRoom2 {
		t.Fatalf("should get the same chatroom for same id - id[%s]", id2)
	}

	if newChatRoom1 == newChatRoom2 {
		t.Fatalf("new chatrooms should not references other chatrooms")
	}

	_, err = messageHub.NewChatRoom(id1)
	if err == nil {
		t.Fatalf("should return an error for creating a chatroom with same id")
	}

	_, err = messageHub.GetChatRoom("invalid")
	if err == nil {
		t.Fatalf("should return an error for getting non existant chatroom")
	}
}
