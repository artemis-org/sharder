package gateway

import (
	"github.com/artemis-org/sharder/utils"
	"log"
	"sync"
	"time"
)

var(
	lastLogin int64
	loginQueue []*Shard
	loginMutex sync.Mutex
)

func StartLoginQueue() {
	go func() {
		for {
			loginMutex.Lock()
			if len(loginQueue) > 0 {

				var s *Shard
				s, loginQueue = loginQueue[len(loginQueue)-1], loginQueue[:len(loginQueue)-1]

				if lastLogin != 0 && lastLogin > utils.GetTimeMillis() - 6000 {
					remaining := 6000 - (utils.GetTimeMillis() - lastLogin)
					time.Sleep(time.Duration(remaining) * time.Millisecond)
				}

				s.IdentifyChan <- s.Identify()
			}
			loginMutex.Unlock()

			time.Sleep(200 * time.Millisecond)
		}
	}()
}

func (s *Shard) QueueIdentify() {
	loginMutex.Lock()
	defer loginMutex.Unlock()

	loginQueue = append(loginQueue, s)
}

func (s *Shard) StartEventQueue() {
	go func() {
		s.ListenChan <- struct{}{}

		for {
			select {
			case data := <- s.EventChan:
				err := s.WriteRaw(data); if err != nil {
					log.Println(err.Error())
				}

				s.EventMutex.Lock()
				s.EventCount += 1
				s.EventMutex.Unlock()

				if s.EventCount < 120 {
					s.ListenChan <- struct{}{}
				}

				go func() {
					time.Sleep(time.Minute)

					s.EventMutex.Lock()
					s.EventCount -= 1
					s.EventMutex.Unlock()

					if s.EventCount == 119 {
						s.ListenChan <- struct{}{}
					}
				}()
			}
		}
	}()
}
