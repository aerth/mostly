package mllm

import (
	"slices"
	"time"
	"uuid"
)

// THIS FILE SUBJECT TO CHANGE

// ChatMessage is a [Message] but with additional metadata (for a multi-participant conversation)
//
// Is a gorm model, (without uint ID)
//
// ChatMessage API is subject to change and not stablized yet
type ChatMessage struct {
	MessageID uuid.UUID `gorm:"type:uuid;uniqueKey;default:null;" json:"message_id,omitempty"` // MessageID is unique identifier for this message
	Message             // Base Message
	From      string    `json:"from,omitempty" gorm:"default:null"`             // The name of user/bot who created this message
	CreatedAt time.Time `json:"created_at"`                                     // Created or received at
	Thread    uuid.UUID `json:"thread,omitempty" gorm:"type:uuid;default:null"` // Conversation thread this message belongs to

	Aux any `json:"-" gorm:"-"` // unused by this package
}

func (m *ChatMessage) HandleChat(cr ChatResponse) error {
	m.Message = m.Message.Add(cr.Message)
	return nil
}

type ChatMessageWithID struct {
	ID uint `json:"-" gorm:"primaryKey"` // TODO: cant json id here for [Message2.ToolCallID2] llama.cpp
	ChatMessage
}

// ChatMessages like [Messages] but with additional metadata (multiple multi-participant conversations)
type ChatMessages []ChatMessage

// ToChatMessage with DefaultThread, new MessageID, new timestamp.
//
// Should be called ONCE immediately after stream is finished.
func (m Message) ToChatMessage(from string) *ChatMessage {
	return NewChatMessage(m, from, DefaultThread)
}

// NewChatMessage from (see also [Message.ToChatMessage])
func NewChatMessage(m Message, from string, thread uuid.UUID) *ChatMessage {
	m2 := &ChatMessage{
		MessageID: uuid.New(),
		Message:   m,
		From:      from,
		CreatedAt: time.Now().Truncate(time.Millisecond),
		Thread:    thread,
		Aux:       nil,
	}
	return m2
}

func (m Message) ToThreadMessage(from string, thread uuid.UUID) *ChatMessage {
	m2 := m.ToChatMessage(from)
	m2.Thread = thread
	return m2
}

func (m ChatMessage) Add(m2 ChatMessage) ChatMessage {
	if m.MessageID == uuid.Nil() {
		m.MessageID = m2.MessageID
	}
	m.Message = m.Message.Add(m2.Message)
	if m.From == "" {
		m.From = m2.From
	}
	if !m2.CreatedAt.IsZero() {
		m.CreatedAt = m2.CreatedAt
	}
	if m.Thread == uuid.Nil() {
		m.Thread = m2.Thread
	}
	if m2.Aux != nil {
		m.Aux = m2.Aux
	}
	return m
}

func (cm ChatMessages) TopP() *ChatMessage {
	l := len(cm)
	if l == 0 {
		return nil
	}
	return &cm[l-1]
}

func (cm ChatMessages) IndexOf(id uuid.UUID) int {
	return slices.IndexFunc(cm, func(e ChatMessage) bool {
		return e.MessageID == id
	})
}

func (cm ChatMessages) ID(id uuid.UUID) *ChatMessage {
	i := cm.IndexOf(id)
	if i < 0 {
		return nil
	}
	return &cm[i]
}

func (cm ChatMessages) Top() ChatMessage {
	l := len(cm)
	if l == 0 {
		return ChatMessage{}
	}
	return cm[l-1]
}

func (cm ChatMessages) TopN(n int) ChatMessages {
	l := len(cm)
	if l == 0 {
		return nil
	}
	if l <= n {
		return cm
	}
	return cm[l-n:]
}

func (cm ChatMessages) TopRole(role string) ChatMessage {
	l := len(cm)
	if l == 0 {
		return ChatMessage{}
	}
	l -= 1
	for i := range cm {
		if cm[l-i].Role == role {
			return cm[i]
		}
	}
	return ChatMessage{}
}

func (cm ChatMessages) ToMessages() Messages {
	a := make(Messages, len(cm))
	for i := range cm {
		a[i] = cm[i].Message
	}
	return a
}

// ChatMessageFormatter may skip a message by returning false
//
// The returned formatted messages are NEVER stored only sent
type ChatMessageFormatter = func(cm ChatMessage) ([]Message, bool)

// ToMessagesFormat returns a **temporary** convo for llm to produce a single new [Message]
// The returned convo should NEVER be stored (only sent to llm) as the Content is modified by formatter.
func (cm ChatMessages) ToMessagesFormat(fn ChatMessageFormatter) []Message {
	if fn == nil {
		fn = DefaultChatMessageFormatter
	}
	var a []Message
	for i := range cm {
		m, ok := fn(cm[i])
		if ok {
			a = append(a, m...)
		}
	}
	return a
}

// DefaultChatMessageFormatter pass through [ChatMessageFormatter]
// In a custom implementation, this might add text to Content, replace Role, or insert messages before/after.
var DefaultChatMessageFormatter = func(cm ChatMessage) ([]Message, bool) {
	return []Message{cm.Message}, true
}
var _ ChatMessageFormatter = DefaultChatMessageFormatter
