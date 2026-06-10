package main

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// CountButtons unchanged except input type changed to *packet.ModalFormRequest (same)

// ModalFormRequest and Button stay the same
type ModalForm struct {
	Buttons []Button `json:"buttons"`
	Content string   `json:"content"`
	Title   string   `json:"title"`
	Type    string   `json:"type"`
}

type Button struct {
	Rank      string `json:"rank"`
	OwnerOnly bool   `json:"ownerOnly"`
	LobbyOnly bool   `json:"lobbyOnly"`
	Text      string `json:"text"`
}

func decodeForm(data []byte) (*ModalForm, error) {
	var form ModalForm
	err := json.Unmarshal(data, &form)
	if err != nil {
		return nil, err
	}

	return &form, nil
}

// refactor pressModalButton to use *Client instead of *minecraft.Conn
func pressModalButton(client *Client, formid uint32, buttonIndex int8) {
	c, err := json.Marshal(buttonIndex)
	if err == nil {
		if err := client.conn.WritePacket(&packet.ModalFormResponse{
			FormID:       formid,
			ResponseData: protocol.Option(c),
		}); err != nil {
			log.Fatalf("Failed to send response %v", err)
		}
	}
}

// Helper function to extract code from raw JSON (unchanged)
func extractCode(rawJSON string) string {
	var form ModalForm
	err := json.Unmarshal([]byte(rawJSON), &form)
	if err != nil {
		panic(err)
	}

	re := regexp.MustCompile(`/cs (AS|EU)_[a-zA-Z0-9]+`)
	match := re.FindString(form.Content)
	if match != "" {
		return match
	}
	return "Access Code not found"
}
func CloseForm(client *Client, formid uint32) {
	if err := client.conn.WritePacket(&packet.ModalFormResponse{
		FormID: formid,
	}); err != nil {
		log.Printf("Failed to close form: %v", err)
	}
	client.data.formOpen = false
}

func CountButtons(p *packet.ModalFormRequest) int {
	var data struct {
		Buttons []struct {
			Text string `json:"text"`
		} `json:"buttons"`
	}

	err := json.Unmarshal([]byte(p.FormData), &data)
	if err != nil {
		log.Printf("[CountButtons] Invalid JSON or not a button form: %s | FormId: %d | Error: %v", p.FormData, p.FormID, err)
		return 0
	}

	return len(data.Buttons)
}

func DecodeButtons(index int, p *packet.ModalFormRequest) string {
	type Button struct {
		Text string `json:"text"`
	}
	var data struct {
		Buttons []Button `json:"buttons"`
		Title   string   `json:"title"`
	}

	err := json.Unmarshal([]byte(p.FormData), &data)
	if err != nil {
		log.Printf("[DecodeButtons] Invalid JSON or not a button form: %s | FormId: %d | Error: %v", p.FormData, p.FormID, err)
		return ""
	}

	if len(data.Buttons) == 0 {
		log.Printf("[DecodeButtons] No buttons found in form: %s | FormId: %d", data.Title, p.FormID)
		//	log.Printf(string(p.FormData))
		return ""
	}

	if index >= 0 && index < len(data.Buttons) {
		log.Printf("[DecodeButtons] Button Label: %s | Form: %s [%d]", data.Buttons[index].Text, data.Title, p.FormID)
		return data.Buttons[index].Text
	}

	log.Printf("[DecodeButtons] Index %d out of range (max %d) in form: %s | FormId: %d", index, len(data.Buttons)-1, data.Title, p.FormID)
	return ""
}

func extractTitle(p *packet.ModalFormRequest) string {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(p.FormData), &data)
	if err != nil {
		log.Printf("Failed to parse form data: %v", err)
		return "Failed to decode form"
	}
	if title, ok := data["title"].(string); ok {
		return title
	}
	return "Title not found"
}

func extractBetween(s, start, end string) string {
	startIndex := strings.Index(s, start)
	if startIndex == -1 {
		return ""
	}
	startIndex += len(start)

	endIndex := strings.Index(s[startIndex:], end)
	if endIndex == -1 {
		return ""
	}

	return s[startIndex : startIndex+endIndex]
}
