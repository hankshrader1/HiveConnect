package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

type API struct {
	mu      sync.RWMutex
	clients []*Client
}

func NewAPI(clients []*Client) *API {
	return &API{clients: clients}
}

func (a *API) Start(port string) {
	mux := http.NewServeMux()

	mux.HandleFunc("/clients", a.getClients)
	mux.HandleFunc("/clients/count", a.getClientCount)
	mux.HandleFunc("/clients/textcommand", a.runChatCommand)
	mux.HandleFunc("/clients/replay", a.replayTrack)

	log.Printf("API listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("API error: %v", err)
	}
}

func (a *API) getClients(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	type clientInfo struct {
		Username string `json:"username"`
		InCustom bool   `json:"in_custom"`
	}

	var info []clientInfo
	for _, c := range a.clients {
		info = append(info, clientInfo{
			Username: c.data.username,
			InCustom: c.data.inCustom,
		})
	}

	json.NewEncoder(w).Encode(info)
}

func (a *API) replayTrack(w http.ResponseWriter, r *http.Request) {
	// Only allow POST
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Define expected body
	var body struct {
		Username string `json:"username"`
		ReplayId string `json:"replayId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, c := range a.clients {
		if c.data.username == body.Username {

			result := ReplayTrack(c, "ImpeccableMap2055")
			if result == true {
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			} else {
				http.Error(w, "Invalid command on Hive", http.StatusBadRequest)
				return
			}

		}
		continue
	}

	http.Error(w, "client not found", http.StatusNotFound)

}
func (a *API) runChatCommand(w http.ResponseWriter, r *http.Request) {
	// Only allow POST
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Define expected body
	var body struct {
		Username      string `json:"username"`
		Command       string `json:"command"`
		ModalExpected bool   `json:"modal"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, c := range a.clients {
		if c.data.username == body.Username {

			result := DelayedCommand(c, body.Command, body.ModalExpected)

			if result == true {
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			} else {
				http.Error(w, "Invalid command on Hive", http.StatusBadRequest)
				return
			}

		}
		continue
	}

	http.Error(w, "client not found", http.StatusNotFound)

}
func (a *API) getClientCount(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]int{
		"count": len(a.clients),
	})
}
