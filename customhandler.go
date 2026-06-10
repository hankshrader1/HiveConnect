package main

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var CsInformation struct {
	clientUsername string
	players        []string
	teamOne        []string
	teamTwo        []string

	setPreset bool
	teamsOf   int

	code        string
	selectedMap string
}

// creates the custom server
func createCustomServer(client *Client) bool {
	if client.data.inCustom {
		return false
	}

	client.conn.WritePacket(&packet.CommandRequest{
		CommandLine: "/cs",
	})

	// First two modals: press button 0
	for range 2 {
		if !waitAndPressButton(client, 5*time.Second, 0, false) {
			return false
		}
	}

	// Final modal: press button 2, then wait for dimension change
	if !waitAndPressButton(client, 5*time.Second, 2, false) {
		return false
	}
	_, ok := client.WaitForPacket("DimensionChangeEnd", 4*time.Second)
	if ok {
		return setupCustomServer(client)
	} else {
		return false
	}
}

// sets up the main roles, preset code and extracts the code
func setupCustomServer(client *Client) bool {
	client.conn.WritePacket(&packet.CommandRequest{
		CommandLine: "/controls",
	})

	// Step 1: Initial modal — press button 2
	if !waitAndPressButton(client, 5*time.Second, 2, false) {
		return false
	}

	// Step 2: Custom form — submit password
	pk, ok := client.WaitForPacket("ModalRequest", 5*time.Second)
	if !ok {
		log.Print("timed out")
		return false
	}
	p := pk.(*packet.ModalFormRequest)
	answers := []interface{}{"bv7pDD"}
	c, err := json.Marshal(answers)
	if err != nil {
		return false
	}
	client.conn.WritePacket(&packet.ModalFormResponse{
		FormID:       p.FormID,
		ResponseData: protocol.Option(c),
	})

	// Steps 3–8: Sequential button presses
	buttonSequence := []int8{0, 5, 0, 2, 0, 0}
	timeouts := []time.Duration{
		5 * time.Second,
		2 * time.Second,
		2 * time.Second,
		2 * time.Second,
		2 * time.Second,
		2 * time.Second,
	}

	for i, btn := range buttonSequence {
		if !waitAndPressButton(client, timeouts[i], btn, i == len(buttonSequence)-1) {
			return false
		}
	}
	client.conn.WritePacket(&packet.CommandRequest{
		CommandLine: "/controls",
	})
	waitAndPressButton(client, time.Second*2, 1, false)
	waitAndPressButton(client, time.Second*2, 2, false)
	waitAndPressButton(client, time.Second*2, 2, false)

	pkt, ok := client.WaitForPacket("ModalRequest", time.Second*2)
	if !ok {
		return false
	} else {
		p := pkt.(*packet.ModalFormRequest)
		if strings.Contains(strings.ToLower(string(p.FormData)), "§bcode-only") {
			CsInformation.code = extractCode(string(p.FormData))

		}
	}
	return true
}

//set teams
//set map

//invite personal
//start cs
