package main

import (
	"log"
	"net/http"
	"trace"

	"github.com/gorilla/websocket"
	"github.com/stretchr/objx"
)

const (
	socketBufferSize  = 1024
	messageBufferSeze = 256
)

var upgrader = &websocket.Upgrader{ReadBufferSize: socketBufferSize,
	WriteBufferSize: socketBufferSize}

type room struct {
	// forward is a channel that holds incoming messages
	// that should be forworded to the other clients.
	forward chan *message
	// join is a channel for clients wishing to join the room.
	join chan *client
	// leave is a channel for clients wishing to leaving the room.
	leave chan *client
	// clients holds all current clients in this room.
	clients map[*client]bool
	// tracer will receive trace information of activity
	// in the room
	tracer trace.Tracer
}

func newRoom() *room {
	return &room{
		forward: make(chan *message),
		join:    make(chan *client),
		leave:   make(chan *client),
		clients: make(map[*client]bool),
		tracer:  trace.Off(),
	}
}

func (r *room) run() {
	for {
		select {
		case client := <-r.join:
			// joining
			r.clients[client] = true
			r.tracer.Trace("New client joined")
		case client := <-r.leave:
			// leaving
			delete(r.clients, client)
			close(client.send)
			r.tracer.Trace("Client left")
		case msg := <-r.forward:
			r.tracer.Trace("Message received: ", msg.Message)
			// forward message to all clients
			for c := range r.clients {
				c.send <- msg
				r.tracer.Trace("-- sent to client")
			}
		}
	}
}

func (r *room) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	socket, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Fatalln("ServeHTTP:", err)
		// return  / why needs a return here???
	}
	authCookie, err := req.Cookie("auth")
	if err != nil {
		log.Fatalln("Failed to get auth cookie:", err)
		// return // no need
	}
	client := &client{
		socket:   socket,
		send:     make(chan *message, messageBufferSeze),
		room:     r,
		userData: objx.MustFromBase64(authCookie.Value),
	}
	r.join <- client
	defer func() {
		r.leave <- client
	}()
	go client.write()
	client.read()
}
