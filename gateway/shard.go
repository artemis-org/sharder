package gateway

import (
	"encoding/json"
	"fmt"
	"github.com/artemis-org/sharder/utils"
	"log"
	"sync"
	"time"

	"github.com/artemis-org/sharder/gateway/payloads"
	"github.com/artemis-org/sharder/gateway/payloads/events"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

type State int

const (
	Dead State = iota
	Connecting
	Disconnecting
	Connected
)

// These are the events to be sent to the cache
var CachedEvents = []string{
	"CHANNEL_CREATE",
	"CHANNEL_UPDATE",
	"CHANNEL_DELETE",
	"GUILD_CREATE",
	"GUILD_UPDATE",
	"GUILD_DELETE",
	"GUILD_EMOJIS_UPDATE",
	"GUILD_MEMBER_ADD",
	"GUILD_MEMBER_REMOVE",
	"GUILD_MEMBER_UPDATE",
	"GUILD_MEMBERS_CHUNK",
	"GUILD_ROLE_CREATE",
	"GUILD_ROLE_UPDATE",
	"GUILD_ROLE_DELETE",
	"VOICE_STATE_UPDATE",
}

type Shard struct {
	Id                int
	State             State
	StateMutex        sync.Mutex
	WebSocket         *websocket.Conn
	ReadMutex         sync.Mutex
	SessionId         string
	SequenceMutex     sync.Mutex
	SequenceNumber    int
	HeartbeatInterval int
	HeartbeatMutex    sync.Mutex
	LastHeartbeat     int64
	IdentifyChan      chan error
	KillHeartbeat     chan struct{}
	RedisClient       *RedisClient

	EventCount  int
	EventChan   chan []byte
	EventMutex  sync.Mutex
	ListenChan  chan struct{}
	SocketMutex sync.Mutex
}

func NewShard(id int, client *RedisClient, sessId string) Shard {
	return Shard{
		Id:             id,
		State:          Dead,
		StateMutex:     sync.Mutex{},
		ReadMutex:      sync.Mutex{},
		SessionId:      sessId,
		SequenceMutex:  sync.Mutex{},
		HeartbeatMutex: sync.Mutex{},
		RedisClient:    client,

		EventChan:   make(chan []byte),
		EventMutex:  sync.Mutex{},
		ListenChan:  make(chan struct{}),
		SocketMutex: sync.Mutex{},
	}
}

func (s *Shard) Connect() error {
	log.Println(fmt.Sprintf("Starting shard %d", s.Id))

	s.StateMutex.Lock()
	if s.State != Dead {
		return s.Kill()
	}

	s.State = Connecting

	conn, _, err := websocket.DefaultDialer.Dial("wss://gateway.discord.gg/?v=6&encoding=json", nil)
	if err != nil {
		s.State = Dead
		return err
	}
	s.StateMutex.Unlock()

	s.WebSocket = conn
	conn.SetCloseHandler(s.onClose)

	err = s.Read()
	if err != nil {
		log.Println(err.Error())
	}

	if s.SessionId == "" {
		s.IdentifyChan = make(chan error)
		s.QueueIdentify()
		err = <-s.IdentifyChan
		if err != nil {
			_ = s.Kill()
			return err
		}
	} else {
		err = s.Resume()
		if err != nil {
			s.SequenceMutex.Lock()
			s.SequenceNumber = 0
			s.SequenceMutex.Unlock()

			s.SessionId = ""

			_ = s.Kill()
			return err
		}
	}

	s.StateMutex.Lock()
	s.State = Connected
	s.StateMutex.Unlock()

	s.Listen()
	s.StartEventQueue()

	go func() {
		s.StateMutex.Lock()
		state := s.State
		s.StateMutex.Unlock()

		for state == Connected {
			err := s.Read()
			if err != nil {
				if err = s.Kill(); err != nil {
					log.Println(err.Error())
				}
				fmt.Println("Reconnecting")
				s.Connect()
			}
		}
	}()

	return nil
}

func (s *Shard) Read() error {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic on reading")

			if err := s.Kill(); err != nil {
				log.Println(err.Error())
			}
			fmt.Println("Reconnecting")
			s.Connect()
		}
	}()

	if s.WebSocket == nil {
		return errors.New("WebSocket is nil")
	}

	s.ReadMutex.Lock()
	_, data, err := s.WebSocket.ReadMessage()
	s.ReadMutex.Unlock()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	payload, err := payloads.NewPayload(data)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	if payload.SequenceNumber != nil {
		s.SequenceMutex.Lock()
		s.SequenceNumber = *payload.SequenceNumber
		s.SequenceMutex.Unlock()
	}

	switch payload.Opcode {
	case 0:
		{ // Event
			event, err := events.NewEvent(data)
			if err != nil {
				log.Println(err.Error())
				return err
			}

			if event.EventName == "READY" {
				ready, err := events.NewReady(data)
				if err != nil {
					log.Println(err.Error())
					return err
				}

				s.RedisClient.SetSessionId(s.Id, ready.ReadyData.SessionId)
				s.SessionId = ready.ReadyData.SessionId
			}

			if utils.Contains(CachedEvents, event.EventName) {
				go s.RedisClient.UpdateCache(string(data))
			}

			go s.RedisClient.QueueEvent(string(data))
		}
	case 9:
		{ // Invalid session
			err := s.Identify()
			if err != nil {
				return err
			}
			if s.SessionId != "" {
				err := s.Resume()
				if err != nil {
					_ = s.Kill()
					s.start()
					return err
				}
			}
		}
	case 10:
		{ // Hello
			hello, err := payloads.NewHello(data)
			if err != nil {
				log.Println(err.Error())
				return err
			}

			s.HeartbeatInterval = hello.EventData.Interval

			ticker := time.NewTicker(time.Duration(int32(s.HeartbeatInterval)) * time.Millisecond)
			s.KillHeartbeat = make(chan struct{})
			go func() {
				for {
					select {
					case <-ticker.C:
						s.HeartbeatMutex.Lock()
						if s.LastHeartbeat != 0 && (time.Now().UnixNano()/int64(time.Millisecond))-int64(s.HeartbeatInterval) > s.LastHeartbeat {
							log.Println(fmt.Sprintf("Shard %d: Didn't receive acknowledgement, restarting", s.Id))
							_ = s.Kill()
							s.KillHeartbeat <- struct{}{}
							s.start()
						}
						s.HeartbeatMutex.Unlock()
						s.Heartbeat()
					case <-s.KillHeartbeat:
						ticker.Stop()
						break
					}
				}
			}()
		}
	case 11:
		{ // HeartbeatAck
			_, err := payloads.NewHeartbeackAck(data)
			if err != nil {
				log.Println(err.Error())
				return err
			}

			s.HeartbeatMutex.Lock()
			s.LastHeartbeat = time.Now().UnixNano() / int64(time.Millisecond)
			s.HeartbeatMutex.Unlock()
		}
	}

	return nil
}

func (s *Shard) Write(payload interface{}) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Mutexes not used here because they're used in WriteRaw
	return s.WriteRaw(encoded)
}

func (s *Shard) WriteRaw(data []byte) error {
	if s.WebSocket == nil {
		msg := fmt.Sprintf("Shard %d: WS is closed", s.Id)
		log.Println(msg)
		return errors.New(msg)
	}

	s.SocketMutex.Lock()
	err := s.WebSocket.WriteMessage(1, data)
	s.SocketMutex.Unlock()

	return err
}

func (s *Shard) Heartbeat() {
	s.SequenceMutex.Lock()
	payload := payloads.NewHeartbeat(s.SequenceNumber)
	s.SequenceMutex.Unlock()

	err := s.Write(payload)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func (s *Shard) Identify() error {
	payload := payloads.NewIdentify(s.Id)
	return s.Write(payload)
}

func (s *Shard) Resume() error {
	s.SequenceMutex.Lock()
	payload := payloads.NewResume(s.SessionId, s.SequenceNumber)
	s.SequenceMutex.Unlock()
	return s.Write(payload)
}

func (s *Shard) Kill() error {
	log.Println(fmt.Sprintf("Killing shard %d", s.Id))

	s.StateMutex.Lock()
	s.State = Disconnecting
	s.StateMutex.Unlock()

	var err error
	if s.WebSocket != nil {
		err = s.WebSocket.Close()
	}

	s.WebSocket = nil

	s.StateMutex.Lock()
	s.State = Dead
	s.StateMutex.Unlock()

	return err
}

func (s *Shard) onClose(code int, text string) error {
	fmt.Println(fmt.Sprintf("Shard %d: Discord closed WS", s.Id))

	if code == 1000 || code == 1001 || code == 4007 { // Closed gracefully || Invalid seq
		s.SessionId = ""

		s.SequenceMutex.Lock()
		s.SequenceNumber = 0
		s.SequenceMutex.Unlock()

		s.RedisClient.SetSessionId(s.Id, "")
	}

	s.KillHeartbeat <- struct{}{}

	_ = s.Kill()
	s.start()
	return nil
}

func (s *Shard) start() {
	err := s.Connect()
	if err != nil {
		fmt.Println(err.Error())
		s.start()
	}
}
