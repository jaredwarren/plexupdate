package hub

import "encoding/json"

// Marshaler ...
type Marshaler interface {
	Marshal() ([]byte, error)
	Unmarshal(data []byte) error
}

// Message ...
type Message struct {
	Type   string      `json:"type,omitempty"`
	Action string      `json:"action,omitempty"`
	Text   string      `json:"text,omitempty"`
	Data   interface{} `json:"data,omitempty"`
	Sender string      `json:"sender,omitempty"`
}

// Marshal ...
func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// Unmarshal ...
func (m *Message) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

// NewMessage ...
func NewMessage(data []byte) (message *Message) {
	message = &Message{}
	message.Unmarshal(data)
	return
}
