package wsterm

import (
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh/terminal"
	"os"
)

type WebTerm struct {
	SocketConn *websocket.Conn
	ttyState   *terminal.State
	errChn     chan error
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

func (w *WebTerm) SetRawtty(isRaw bool) {
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

func (w *WebTerm) Run() {
	w.errChn = make(chan error)
	w.SetRawtty(true)

	go w.wsRead()
	go w.wsWrite()

	err := <-w.errChn
	w.SetRawtty(false)

	if err != nil {
		panic(err)
	}
}
