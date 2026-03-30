package main

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	clients   = map[string]map[string]*ClientConn{} // lobby -> playerTag -> conn
	clientsMu sync.RWMutex
	upgrader  = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
)

type ClientConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func (c *ClientConn) send(msg []byte) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	c.conn.WriteMessage(websocket.TextMessage, msg)
}

func WSEndpoint(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &ClientConn{conn: conn}
	var lobby, playerTag string
	defer func() {
		conn.Close()
		if lobby != "" {
			removeClient(lobby, playerTag)
		}
	}()

	for {
		var msg struct {
			Type   string `json:"type"`
			Player string `json:"player"`
			Lobby  string `json:"lobby"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		if msg.Type != "me" {
			continue
		}

		if lobby != "" {
			removeClient(lobby, playerTag)
		}
		lobby, playerTag = msg.Lobby, msg.Player
		registerClient(lobby, playerTag, client)

		lobbiesMu.RLock()
		if game := lobbies[lobby]; game != nil {
			b, _ := json.Marshal(struct {
				Type string `json:"type"`
				Game *Game  `json:"game"`
			}{"state", game})
			client.send(b)
		}
		lobbiesMu.RUnlock()
	}
}

func registerClient(lobby, player string, c *ClientConn) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if clients[lobby] == nil {
		clients[lobby] = map[string]*ClientConn{}
	}
	if old, ok := clients[lobby][player]; ok {
		old.conn.Close()
	}
	clients[lobby][player] = c
}

func removeClient(lobby, player string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	delete(clients[lobby], player)
	if len(clients[lobby]) == 0 {
		delete(clients, lobby)
	}
}

// PreparePlayerUpdates serializes changed players to JSON while the lobby lock is held.
func PreparePlayerUpdates(lobbyCode string, changedTags []string) [][]byte {
	game := lobbies[lobbyCode]
	if game == nil {
		return nil
	}
	var msgs [][]byte
	for _, tag := range changedTags {
		for _, p := range game.Players {
			if p.Tag == tag {
				if b, err := json.Marshal(struct {
					Type   string  `json:"type"`
					Player *Player `json:"player"`
				}{"update", p}); err == nil {
					msgs = append(msgs, b)
				}
				break
			}
		}
	}
	return msgs
}

// BroadcastPrepared sends pre-serialized messages to all clients in a lobby.
func BroadcastPrepared(lobbyCode string, msgs [][]byte) {
	clientsMu.RLock()
	conns := make([]*ClientConn, 0, len(clients[lobbyCode]))
	for _, c := range clients[lobbyCode] {
		conns = append(conns, c)
	}
	clientsMu.RUnlock()

	for _, c := range conns {
		for _, msg := range msgs {
			c.send(msg)
		}
	}
}
