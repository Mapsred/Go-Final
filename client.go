package main

import (
	"bytes"
	"log"
	"net/http"
	"time"
	"github.com/gorilla/websocket"
	"encoding/json"
	"strings"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub  *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// DataContent is the JSON who comes from the JavaScript websocket action
type DataContent struct {
	Username string
	Message  string
	File     string
}

// Slice containing current users
var Users []string = make([]string, 0)

// readPump pumps messages from the websocket connection to the hub.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		c.hub.broadcast <- message
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			var data DataContent
			err := json.Unmarshal(message, &data)
			if err != nil {
				log.Printf("Unmarshal: %v", err)
			}

			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if data.Message != "null" {
				w.Write([]byte(buildMessage(data)))
				w.Write(sendUsers(data, false))
			} else {
				w.Write(sendUsers(data, true))
			}

		// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client
	go client.writePump()
	client.readPump()
}

//Build the html message
func buildMessage(content DataContent) string {
	if !stringInSlice(content.Username, Users) {
		Users = append(Users, content.Username)
	}
	var message = content.Username + " : " + content.Message
	str := strings.Replace(content.File, "C:\\fakepath\\", "", -1)
	if str != "" {
		message += " <a target='about:blank' href='http://localhost/golang/" + str + "'>" + str + "</a>"
	}

	return message
}
//Helper method to check if a given string exists in a given slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

//Helper method to remove a given string exists from a given slice
func removeInSlice(a string, list []string) []string {
	var final []string = make([]string, 0)
	for _, item := range list {
		if a != item {
			final = append(final, item)
		}
	}
	return final
}

//Send current user list
func sendUsers(data DataContent, deleting bool) []byte {
	if (deleting) {
		Users = removeInSlice(data.Username, Users)
	}
	users, _ := json.Marshal(append([]string{"USER LIST"}, Users...))

	return users
}