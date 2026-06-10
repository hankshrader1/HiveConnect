package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/oauth2"
)

type PacketListener struct {
	result chan packet.Packet
}
type Client struct {
	conn *minecraft.Conn

	data *ClientData

	listenersMu     sync.Mutex
	packetListeners map[string]*PacketListener
}

type TokenFile struct {
	Tokens []oauth2.Token `json:"tokens"`
}

func loadOrRefreshTokens(path string) ([]oauth2.TokenSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tf TokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, err
	}

	var sources []oauth2.TokenSource
	changed := false

	for i, token := range tf.Tokens {
		t := token
		src := auth.RefreshTokenSource(&t)
		refreshed, err := src.Token()
		if err != nil {
			return nil, fmt.Errorf("token %d: %w", i, err)
		}
		if refreshed.AccessToken != t.AccessToken || refreshed.Expiry != t.Expiry {
			tf.Tokens[i] = *refreshed
			changed = true
		}
		sources = append(sources, auth.RefreshTokenSource(refreshed))
	}

	if changed {
		updated, err := json.MarshalIndent(tf, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, updated, 0644); err != nil {
			return nil, err
		}
	}

	return sources, nil
}

var (
	clientsMu sync.RWMutex
	clients   []*Client
)

func addClient(c *Client) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	clients = append(clients, c)
}

func removeClient(c *Client) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for i, cl := range clients {
		if cl == c {
			clients = append(clients[:i], clients[i+1:]...)
			return
		}
	}
}

func main() {
	const tokenPath = "xbl_token.json"

	sources, err := loadOrRefreshTokens(tokenPath)
	if err != nil {
		panic(err)
	}
	test(sources[0])
	select {}
	// Create and register one client per token, but don't start them
	for _, src := range sources {
		log.Printf("Loading Client")
		client, err := NewClient(src)
		if err != nil {
			log.Printf("Failed to create client: %v", err)
			continue
		}
		addClient(client)

		log.Printf("Registered client, total: %d", len(clients))
	}

	// Now start them all
	clientsMu.RLock()
	for _, c := range clients {
		defer c.conn.Close()
		log.Print(c.data.username)
		//    handleClientPackets(*c)
	}
	clientsMu.RUnlock()
	api := NewAPI(clients)
	go api.Start("8080")
	select {}
}
