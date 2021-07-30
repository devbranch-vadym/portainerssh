package wsterm

import (
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh/terminal"
	"os"
)

// WebTerm connects to remote shell via websocket protocol and connects it to local terminal.
type WebTerm struct {
	SocketConn *websocket.Conn
	ttyState   *terminal.State
	errChn     chan error
}

// NewWebTerm creates a new WebTerm object and connects it to a given websocket.
func NewWebTerm(socketConn *websocket.Conn) *WebTerm {
	return &WebTerm{SocketConn: socketConn}
}

func (w *WebTerm) wsWrite() {
	var err error
	var keybuf [1]byte
	for {
		_, err = os.Stdin.Read(keybuf[0:1])
		if err != nil {
			w.errChn <- err
			return
		}

		err = w.SocketConn.WriteMessage(websocket.BinaryMessage, keybuf[0:1])
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
				w.errChn <- nil
			} else {
				w.errChn <- err
			}
			return
		}
	}
}

func (w *WebTerm) wsRead() {
	var err error
	var raw []byte
	for {
		_, raw, err = w.SocketConn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
				w.errChn <- nil
			} else {
				w.errChn <- err
			}
			return
		}
		os.Stdout.Write(raw)
	}
}

func (w *WebTerm) setRawtty(isRaw bool) {
	if isRaw {
		state, err := terminal.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		w.ttyState = state
	} else {
		terminal.Restore(int(os.Stdin.Fd()), w.ttyState)
	}
}

// Run starts transferring data between local terminal and remote shell connection.
func (w *WebTerm) Run() {
	w.errChn = make(chan error)
	w.setRawtty(true)

	go w.wsRead()
	go w.wsWrite()

	err := <-w.errChn
	w.setRawtty(false)

	if err != nil {
		panic(err)
	}
}
