package cherrySession

import (
	"fmt"
	"github.com/cherry-game/cherry/extend/utils"
	"github.com/cherry-game/cherry/facade"
	"github.com/cherry-game/cherry/logger"
	"net"
	"sync/atomic"
	"time"
)

var (
	IllegalUID = cherryUtils.Error("illegal uid")
)

var (
	SessionStatus = map[int]string{
		Init:    "init",
		WaitAck: "wait_ack",
		Working: "working",
		Closed:  "closed",
	}
)

const (
	Init = iota
	WaitAck
	Working
	Closed
)

type Session struct {
	Settings
	status            int
	conn              net.Conn
	running           bool
	sid               cherryFacade.SID            // session id
	uid               cherryFacade.UID            // user unique id
	frontendId        cherryFacade.FrontendId     // frontend node id
	net               cherryFacade.INetworkEntity // network opts
	sessionComponent  *SessionComponent           // session SessionComponent
	lastTime          int64                       // last update time
	sendChan          chan []byte
	onCloseListeners  []cherryFacade.SessionListener
	onErrorListeners  []cherryFacade.SessionListener
	onMessageListener cherryFacade.MessageListener
}

func NewSession(sid cherryFacade.SID, conn net.Conn, net cherryFacade.INetworkEntity, sc *SessionComponent) *Session {
	session := &Session{
		Settings: Settings{
			data: make(map[string]interface{}),
		},
		status:           Init,
		conn:             conn,
		running:          false,
		sid:              sid,
		uid:              0,
		frontendId:       "",
		net:              net,
		sessionComponent: sc,
		lastTime:         time.Now().Unix(),
		sendChan:         make(chan []byte),
	}

	session.onCloseListeners = append(session.onCloseListeners, func(session cherryFacade.ISession) {
		cherryLogger.Debugf("on closed. session:%s", session)
	})

	session.onErrorListeners = append(session.onErrorListeners, func(session cherryFacade.ISession) {
		cherryLogger.Debugf("on error. session:%s", session)
	})

	return session
}

func (s *Session) SID() cherryFacade.SID {
	return s.sid
}

func (s *Session) UID() cherryFacade.UID {
	return s.uid
}

func (s *Session) FrontendId() cherryFacade.FrontendId {
	return s.frontendId
}

func (s *Session) SetStatus(status int) {
	s.status = status
}

func (s *Session) Status() int {
	return s.status
}

func (s *Session) Net() cherryFacade.INetworkEntity {
	return s.net
}

func (s *Session) Bind(uid cherryFacade.UID) error {
	if uid < 1 {
		return IllegalUID
	}

	atomic.StoreInt64(&s.uid, uid)
	return nil
}

func (s *Session) Conn() net.Conn {
	return s.conn
}

func (s *Session) OnClose(listener cherryFacade.SessionListener) {
	if listener != nil {
		s.onCloseListeners = append(s.onCloseListeners, listener)
	}
}

func (s *Session) OnError(listener cherryFacade.SessionListener) {
	if listener != nil {
		s.onErrorListeners = append(s.onErrorListeners, listener)
	}
}

func (s *Session) OnMessage(listener cherryFacade.MessageListener) {
	if listener != nil {
		s.onMessageListener = listener
	}
}

func (s *Session) Send(msg []byte) error {
	if !s.running {
		return nil
	}

	s.sendChan <- msg
	return nil
}

func (s *Session) SendBatch(batchMsg ...[]byte) {
	for _, msg := range batchMsg {
		s.Send(msg)
	}
}

func (s *Session) Start() {
	s.running = true

	// read goroutine
	go s.readPackets(2048)

	for s.running {
		select {
		case msg := <-s.sendChan:
			_, err := s.conn.Write(msg)
			if err != nil {
				s.Closed()
			}
		}
	}
}

// readPackets read connection data stream
func (s *Session) readPackets(readSize int) {
	if s.onMessageListener == nil {
		panic("onMessageListener() not set.")
	}

	defer func() {
		for _, listener := range s.onCloseListeners {
			listener(s)
		}

		//close connection
		err := s.conn.Close()
		if err != nil {
			cherryLogger.Error(err)
		}
	}()

	buf := make([]byte, readSize)

	for s.running {
		n, err := s.conn.Read(buf)
		if err != nil {
			cherryLogger.Warnf("read message error: %s, socket will be closed immediately", err.Error())
			for _, listener := range s.onErrorListeners {
				listener(s)
			}
			s.running = false
			return
		}

		if n < 1 {
			continue
		}

		// (warning): decoder use slice for performance, packet data should be copy before next PacketDecode
		err = s.onMessageListener(buf[:n])
		if err != nil {
			cherryLogger.Warn(err)
			return
		}
	}
}

func (s *Session) Closed() {
	s.status = Closed
	s.running = false

	if s.sessionComponent != nil {
		s.sessionComponent.Remove(s.sid)
	}
}

func (s *Session) String() string {
	return fmt.Sprintf("sid = %d, uid = %d, status=%s, address = %s, running = %v",
		s.sid,
		s.uid,
		SessionStatus[s.status],
		s.conn.RemoteAddr().String(),
		s.running)
}
