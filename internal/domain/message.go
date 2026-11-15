package domain

type MessageRole int

const (
	MessageRoleUser MessageRole = iota
	MessageRoleAssistant
)

func (mr MessageRole) IsValid() bool {
	switch mr {
	case MessageRoleUser, MessageRoleAssistant:
		return true
	default:
		return false
	}
}

type Message struct {
	ID      int64
	Role    MessageRole
	Content string
}

func NewMessage(role MessageRole, content string) Message {
	return Message{
		Role:    role,
		Content: content,
	}
}
