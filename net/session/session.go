package cherrySession

import (
	"fmt"
	"github.com/cherry-game/cherry/error"
	facade "github.com/cherry-game/cherry/facade"
	"github.com/cherry-game/cherry/logger"
	"sync/atomic"
)

var nextSessionId int64

func NextSID() facade.SID {
	return atomic.AddInt64(&nextSessionId, 1)
}

type (
	Session struct {
		settings
		entity     facade.INetwork   // network
		sid        facade.SID        // session id
		uid        facade.UID        // user unique id
		frontendId facade.FrontendId // frontend node id
		component  *Component
	}
)

func NewSession(sid facade.SID, frontendId facade.FrontendId, entity facade.INetwork, component *Component) *Session {
	session := &Session{
		settings: settings{
			data: make(map[string]interface{}),
		},
		entity:     entity,
		sid:        sid,
		uid:        0,
		frontendId: frontendId,
		component:  component,
	}

	if component != nil {
		for _, listener := range component.onCreate {
			if listener(session) == false {
				break
			}
		}
	}

	return session
}

func (s *Session) SID() facade.SID {
	return s.sid
}

func (s *Session) UID() facade.UID {
	return s.uid
}

func (s *Session) FrontendId() facade.FrontendId {
	return s.frontendId
}

func (s *Session) IsBind() bool {
	return s.uid > 0
}

func (s *Session) Bind(uid facade.UID) error {
	if uid < 1 {
		return cherryError.SessionIllegalUID
	}

	return s.component.Bind(s.sid, uid)
}

func (s *Session) Unbind() {
	s.component.Unbind(s.sid)
}

func (s *Session) SendRaw(bytes []byte) error {
	if s.entity == nil {
		s.Debug("entity is nil")
		return nil
	}

	return s.entity.SendRaw(bytes)
}

// RPC sends message to remote server
func (s *Session) RPC(route string, v interface{}) error {
	return s.entity.RPC(route, v)
}

// Push message to client
func (s *Session) Push(route string, v interface{}) error {
	return s.entity.Push(route, v)
}

// Response responses message to client, mid is
// request message ID
func (s *Session) Response(mid uint, v interface{}) error {
	return s.entity.Response(mid, v)
}

func (s *Session) Kick(reason string, close bool) {
	err := s.entity.Kick(reason)
	if err != nil {
		s.Warn(err)
		return
	}

	if close {
		s.Close()
	}
}

func (s *Session) Close() {
	// 服务器调用Close()主动断开
	// 客户端主动断开/读取错误断开
	s.entity.Close()
}

func (s *Session) OnCloseProcess() {
	// when session closed,the func is executed
	if s.component != nil {
		for _, listener := range s.component.onClose {
			if listener(s) == false {
				break
			}
		}
	}
	s.Unbind()
}

func (s *Session) RemoteAddress() string {
	if s.entity == nil {
		return ""
	}
	return s.entity.RemoteAddr().String()
}

func (s *Session) String() string {
	return fmt.Sprintf("sid = %d, uid = %d, address = %s",
		s.sid,
		s.uid,
		s.RemoteAddress(),
	)
}

func (s *Session) logPrefix() string {
	return fmt.Sprintf("[sid=%d, uid=%d] ", s.sid, s.uid)
}

func (s *Session) Debug(args ...interface{}) {
	cherryLogger.DefaultLogger.Debug(s.logPrefix(), fmt.Sprint(args...))
}

func (s *Session) Debugf(template string, args ...interface{}) {
	cherryLogger.DefaultLogger.Debug(s.logPrefix(), fmt.Sprintf(template, args...))
}

func (s *Session) Info(args ...interface{}) {
	cherryLogger.DefaultLogger.Info(s.logPrefix(), fmt.Sprint(args...))
}

func (s *Session) Infof(template string, args ...interface{}) {
	cherryLogger.DefaultLogger.Info(s.logPrefix(), fmt.Sprintf(template, args...))
}

func (s *Session) Warn(args ...interface{}) {
	cherryLogger.DefaultLogger.Warn(s.logPrefix(), fmt.Sprint(args...))
}

func (s *Session) Warnf(template string, args ...interface{}) {
	cherryLogger.DefaultLogger.Warn(s.logPrefix(), fmt.Sprintf(template, args...))
}

func (s *Session) Error(args ...interface{}) {
	cherryLogger.DefaultLogger.Error(s.logPrefix(), fmt.Sprint(args...))
}

func (s *Session) Errorf(template string, args ...interface{}) {
	cherryLogger.DefaultLogger.Error(s.logPrefix(), fmt.Sprintf(template, args...))
}
