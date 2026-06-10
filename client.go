package main

import (
	"log"
	"math"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/oauth2"
)

type ClientData struct {
	username          string
	inCustom          bool
	inReplay          bool
	csCount           int
	limitReached      bool
	changingDimension bool
	formOpen          bool
	tick              uint64
}

func NewClient(src oauth2.TokenSource) (*Client, error) {

	dialer := minecraft.Dialer{
		TokenSource: src,
	}

	address := "sg.hivebedrock.network:19132"

	var conn *minecraft.Conn
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		conn, err = dialer.Dial("raknet", address)
		if err == nil {
			break
		}
		log.Printf("Connection attempt %d failed: %v, retrying...", attempts+1, err)
		time.Sleep(time.Second * 2)
	}
	if err != nil {
		panic(err)
	}

	for attempts := 0; attempts < 3; attempts++ {
		err = conn.DoSpawn()
		if err == nil {
			break
		}
		log.Printf("Spawn attempt %d failed: %v, retrying...", attempts+1, err)

	}
	if err != nil {
		panic(err)
	}

	//Connected
conn.WritePacket(&packet.RequestChunkRadius{
	ChunkRadius:  ,
})
	var client = &Client{
		conn: conn,

		data: &ClientData{
			username:          conn.ClientData().ThirdPartyName,
			inCustom:          false,
			inReplay:          false,
			limitReached:      false,
			changingDimension: false,
			formOpen:          false,
		},
	}
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			atomic.AddUint64(&client.data.tick, 1)
		}
	}()
	go handleClientPackets(client)
	log.Printf("Connected on: %s", conn.ClientData().ThirdPartyName)
	client.packetListeners = make(map[string]*PacketListener)
	return client, nil
}

func handleClientPackets(client *Client) {

	var conn = client.conn
	for {
		pkt, err := conn.ReadPacket()
		if err != nil {
			log.Printf("[%s] Read error: %v", client.data.username, err)

			return
		}

		switch p := pkt.(type) {
		case *packet.PlayerAuthInput:
			client.data.tick = p.Tick
			log.Printf("AUTH INPUT: %f, %f, %f", p.Position.X(), p.Position.Y(), p.Position.Z())
		case *packet.MovePlayer:
			log.Printf("MOVE PLAYER: %f, %f, %f", p.Position.X(), p.Position.Y(), p.Position.Z())

		case *packet.Text:
			client.notifyListener("Text", pkt)
		case *packet.ChangeDimension:
			conn.WritePacket(&packet.PlayerAction{ActionType: 14})
		case *packet.PlayerAction:
			{
				if p.ActionType == 14 {
					client.notifyListener("DimensionChangeEnd", pkt)
				}
			}
		case *packet.Disconnect:
			log.Printf("[%s] Disconnected", client.data.username)
		case *packet.ModalFormRequest:
			client.notifyListener("ModalRequest", pkt)
			form, err := decodeForm(p.FormData)
			if err != nil {
				log.Print("Error decoding form")
				continue
			}

			log.Print(form.Title)
		case *packet.AddActor:
			//	log.Print(p.EntityType)
		case *packet.PlayerList:

			if p.ActionType == packet.PlayerListActionAdd {
				/*for _, entry := range p.Entries {

				}*/
			}

		}
		//OnPacket_Movement(conn, pkt)

	}
}

func (c *Client) WaitForPacket(packetType string, timeout time.Duration) (packet.Packet, bool) {
	l := &PacketListener{result: make(chan packet.Packet, 1)}

	c.listenersMu.Lock()
	c.packetListeners[packetType] = l
	c.listenersMu.Unlock()

	defer func() {
		c.listenersMu.Lock()
		delete(c.packetListeners, packetType)
		c.listenersMu.Unlock()
	}()

	select {
	case pk := <-l.result:
		return pk, true
	case <-time.After(timeout):
		return nil, false
	}
}

func (c *Client) notifyListener(typeName string, pk packet.Packet) {
	c.listenersMu.Lock()
	l, ok := c.packetListeners[typeName]
	c.listenersMu.Unlock()
	if ok {
		l.result <- pk
	}
}
func DelayedCommand(client *Client, command string, modalExpected bool) bool {

	log.Printf("[%s] Executing delayed command: %s", client.data.username, command)
	time.Sleep(1 * time.Second)
	_ = client.conn.WritePacket(&packet.CommandRequest{
		CommandLine:   command,
		CommandOrigin: protocol.CommandOrigin{},
	})
	if !modalExpected {
		pk, ok := client.WaitForPacket("Text", 50*time.Millisecond)

		if ok {
			text := pk.(*packet.Text)

			if strings.Contains(strings.ToLower(text.Message), "unknown command") {
				return false
			} else {
				return true
			}
		} else {
			return true
		}

	} else {
		pk, ok := client.WaitForPacket("ModalRequest", 50*time.Millisecond)
		if !ok {
			log.Print("timed out")
			return false
		} else {
			modal := pk.(*packet.ModalFormRequest)
			log.Printf("Modal with Id %v recieved", modal.ID)
		}

	}

	return true
}

func SendMessageThroughBot(client *Client, msg string) {
	_ = client.conn.WritePacket(&packet.Text{
		TextType: packet.TextTypeChat,
		Message:  msg,
	})
}

func ReplayTrack(client *Client, replayId string) bool {
	_ = client.conn.WritePacket(&packet.CommandRequest{
		CommandLine:   "/queue replay",
		CommandOrigin: protocol.CommandOrigin{},
	})

	_, ok := client.WaitForPacket("DimensionChangeEnd", time.Second*25)
	if !ok {
		log.Print("Dimension change failed..?")
		return false
	}

	//p, _ := client.WaitForPacket("Time", time.Millisecond*50)

	client.MoveTo(mgl32.Vec3{0.5, 7.62, 1.5}, -1.48, 7.62, 7.7)

	return true
}
func (c *Client) MoveTo(startPos mgl32.Vec3, targetX, targetY, targetZ float32) {
	go func() {
		targetPos := mgl32.Vec3{targetX, targetY, targetZ}
		diff := targetPos.Sub(startPos)
		steps := int(float32(math.Sqrt(float64(diff[0]*diff[0]+diff[1]*diff[1]+diff[2]*diff[2]))) / 0.18)
		if steps < 1 {
			steps = 1
		}

		yaw := float32(math.Atan2(float64(-diff[0]), float64(diff[2])) * 180 / math.Pi)

		for i := 1; i <= steps; i++ {
			t := float32(i) / float32(steps)
			t = 1 - (1-t)*(1-t)

			prev := startPos
			startPos = mgl32.Vec3{
				targetPos[0]*t + prev[0]*(1-t),
				targetPos[1]*t + prev[1]*(1-t),
				targetPos[2]*t + prev[2]*(1-t),
			}
			log.Printf("%s %s %s", startPos.X(), startPos.Y, startPos.Z())
			c.conn.WritePacket(&packet.PlayerAuthInput{
				Yaw:               yaw,
				HeadYaw:           yaw,
				InteractYaw:       yaw,
				Position:          startPos,
				MoveVector:        mgl32.Vec2{0, 1},
				InputData:         protocol.NewBitset(packet.PlayerAuthInputBitsetSize),
				InputMode:         packet.InputModeTouch,
				PlayMode:          packet.PlayModeNormal,
				InteractionModel:  packet.InteractionModelTouch,
				Tick:              atomic.LoadUint64(&c.data.tick),
				Delta:             startPos.Sub(prev),
				CameraOrientation: mgl32.Vec3{0, 0, 1},
			})
			time.Sleep(50 * time.Millisecond)
		}

		c.conn.WritePacket(&packet.PlayerAuthInput{
			Pitch:             0,
			Yaw:               0,
			HeadYaw:           0,
			InteractPitch:     0,
			InteractYaw:       0,
			Position:          targetPos,
			MoveVector:        mgl32.Vec2{0, 0},
			InputData:         protocol.NewBitset(packet.PlayerAuthInputBitsetSize),
			InputMode:         packet.InputModeTouch,
			PlayMode:          packet.PlayModeNormal,
			InteractionModel:  packet.InteractionModelTouch,
			Tick:              atomic.LoadUint64(&c.data.tick),
			Delta:             mgl32.Vec3{0, 0, 0},
			CameraOrientation: mgl32.Vec3{0, 0, 1},
		})
		time.Sleep(time.Millisecond * 50)
		c.conn.WritePacket(&packet.Interact{
			TargetEntityRuntimeID: 1038,
			ActionType:            4,
			Position:              protocol.Option(mgl32.Vec3{-1.8647398, 7.62001, 9.08})})

		modalRequest, oke := c.WaitForPacket("ModalRequest", time.Second*5)
		if !oke {
			return

		}
		modal := modalRequest.(*packet.ModalFormRequest)
		form, err := decodeForm(modal.FormData)
		if err != nil {
			return
		}
		log.Printf(form.Title)

	}()
}

// waitAndPressButton waits for a ModalFormRequest and presses the given button index.
func waitAndPressButton(client *Client, timeout time.Duration, button int8, close bool) bool {
	pk, ok := client.WaitForPacket("ModalRequest", timeout)
	if !ok {
		log.Print("timed out")
		return false
	}
	p := pk.(*packet.ModalFormRequest)
	pressModalButton(client, p.FormID, button)

	if close {

		pk, _ := client.WaitForPacket("ModalRequest", time.Second*2)
		p := pk.(*packet.ModalFormRequest)
		CloseForm(client, p.FormID)
	}
	return true
}
