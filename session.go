//Sessions are stores of information kept on the server that 
//persist across page loads
//
// this is to be used with Gary Burd's Twister, and functions as a middleware handler
//	server.Run(":8080",
//		SessionHandler(NewMemoryStore(),
//		web.NewRouter().
//		Register("/", "GET", index).
//...
//
//
//data is stored in the session as follows
//	var val int
//	var f float64
//	var s string
//
//	Get(req, "float64", &f)
//	Set(req, "float64", f + .1)
//
//	Get(req, "string", &s)
//	Set(req, "string", s + ".")
//
//	Get(req,"counter2", &val)
//	Set(req, "counter2", val + 1)


package session

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"time"
	"github.com/garyburd/twister/web"
)

const (
	sessionCookieName = "twisterSess"
	sessionValidSeconds = 1440
	sessionSweepSeconds = 600 * 1000000000
)


//the sessionhandler type
type sessionHandler struct {
	h web.Handler
	manager SessionManager
}

//ctor for the sessionhandler, we take a handler and a sessionManager as input params, 
//and return the session handler
func SessionHandler(manager SessionManager, h web.Handler) web.Handler {
	return &sessionHandler{h: h, manager: manager}
}

// the mandatory serveWeb method
func (h *sessionHandler) ServeWeb(req *web.Request) {
	sess := h.manager.Load(req)
	req.Env["session"] = sess

	web.FilterRespond(req, func(status int, header web.Header) (int, web.Header) {
		sess, ok := req.Env["session"].(*Session)
		if !ok {
			return status, header
		}
		h.manager.Save(req, sess)
		
		c := web.NewCookie(sessionCookieName, sess.id).String()
		header.Add(web.HeaderSetCookie, c)
		return status, header
	})
	h.h.ServeWeb(req)
}

//a session manager defines a type of persistant store
//required methods are Load, Save, and Sweep 
type SessionManager interface {
	Load(req *web.Request) *Session
	Save(req *web.Request, sess *Session) bool
	Sweep()
}


//an in-memory session store
//items are stored in a map on the server
type memoryStore struct {
	store map[string]*Session
}

func MemoryStore() *memoryStore {
	ms := &memoryStore{store: make(map[string]*Session)}
	go ms.Sweep()
	return ms
}

func (s *memoryStore) Load(req *web.Request) *Session {
	val := req.Cookie.Get(sessionCookieName)

	sess, ok := s.store[val]
	if !ok {
		sess = NewSession()
	}
	
	return sess
}

func (s *memoryStore) Save(req *web.Request, sess *Session) bool {
	sess.timestamp = time.Seconds()
	s.store[sess.id] = sess
	return true
}

//session stores can accumulate cruft
//you want to be able to sweep the session store, and remove items that are of no further use.
//this means deleting sessions that have a timestamp that is more then sessionValidSeconds old.
func (s *memoryStore) Sweep() {
	for {
		beg := time.Nanoseconds()

		i := 0
		l := len(s.store)
		for k, sess := range s.store {
			if sess.timestamp + sessionValidSeconds < time.Seconds() {
				//this session has expired
				s.store[k] = nil, false
				i++
			}
		}
		taken := time.Nanoseconds() - beg
		

		log.Printf("session store had %d total sessions, but deleted %d sessions. took %v ms",
			l,i, taken/1000000)
		time.Sleep(sessionSweepSeconds)
	}

}
//stores the user data
type Session struct {
	data map[string]interface{}
	id string
	timestamp int64
}

//ctor, returns an initialized session
func NewSession() *Session {
	return &Session{id: uuid(), data: make(map[string]interface{}),timestamp: time.Seconds()}
}


//get information from the store
func Get(req *web.Request, key string, ret interface{})  {
	sess, ok := req.Env["session"].(*Session)
	if !ok {
		return
	}

	val, ok := sess.data[key]
	if !ok {
		return
	}
	
	rv := reflect.ValueOf(ret)

	if rv.Elem().CanSet() {
		rv.Elem().Set(reflect.ValueOf(val))
	}
}
// set a key, value into the session
func Set(req *web.Request, key string, value interface{}) bool {
	sess, ok := req.Env["session"].(*Session)
	if !ok {
		return false
	}

	sess.data[key] = value
	return true
}

// generate a (hopefully) unique session id
func uuid() string {
	f, err := os.Open("/dev/urandom") 
	defer f.Close()
	if err != nil {
		return ""
	}

	b := make([]byte, 16) 
	_, err = f.Read(b) 
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]) 
}
