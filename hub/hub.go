package hub

import (
	"fmt"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// IClient ...
type IClient interface {
	Close()
	Send(msg []byte)
	GetSend() chan []byte
	GetName() string
	SetID(id string)
	GetID() string
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	ID string
	// Registered clients.
	clients map[IClient]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan IClient

	// Unregister requests from clients.
	unregister chan IClient

	index int
}

// Broadcast ...
func (h *Hub) Broadcast(msg Marshaler) {
	data, _ := msg.Marshal()
	h.broadcast <- data
}

// Register ...
func (h *Hub) Register(client IClient) {
	client.SetID(fmt.Sprintf("ID%d", h.index))
	h.index++
	h.register <- client
}

// NewHub ...
func NewHub() *Hub {
	return &Hub{
		ID:         fmt.Sprintf("%d", rand.Intn(100)), // for now
		broadcast:  make(chan []byte),
		register:   make(chan IClient),
		unregister: make(chan IClient),
		clients:    make(map[IClient]bool),
	}
}

// Run ...
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				send := client.GetSend()
				select {
				case send <- message:
					fmt.Println(">>(", client.GetName(), ")", string(message))
				default:
					client.Close()
					delete(h.clients, client)
				}
			}
		}
	}
}

// Close ...
func (h *Hub) Close() {
	for client := range h.clients {
		delete(h.clients, client)
		client.Close()
	}
}
