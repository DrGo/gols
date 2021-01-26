package gols

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/drgo/core/watcher"
	"github.com/gorilla/websocket"
)

const (
	msgBufSize = 1024
)

var (
	// a reusable Upgrader used to upgrade http connections to handle websocket traffic
	upgrader = websocket.Upgrader{
		ReadBufferSize:  msgBufSize,
		WriteBufferSize: msgBufSize,
	}
	msgReload = []byte("reload")
)

// Client handles communication with a browser session
type Client struct {
	ws   *websocket.Conn
	send chan []byte
}

// Read reads and currently throw away messages
// we are not interested in what the client has to say
func (c *Client) Read() {
	for {
		if _, _, err := c.ws.ReadMessage(); err != nil {
			break
		}
	}
	c.ws.Close()
}

// Write sends all messages received by the send chan
// to the client
func (c *Client) Write() {
	for msg := range c.send {
		if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
	c.ws.Close()
}

// Reloader an http handler that reloads its clients
type Reloader struct {
	sync.RWMutex
	clients     map[string]*Client
	toWatch     chan string
	watchEvents chan watcher.Event
}

func NewReloader(ctx context.Context) *Reloader {
	re := &Reloader{
		clients:     make(map[string]*Client),
		toWatch:     make(chan string, 24),
		watchEvents: make(chan watcher.Event, 24),
	}
	//launch watcher
	go watcher.Watch(ctx, re.toWatch, re.watchEvents)
	// launch watch events handler
	go func() {
		for {
			select {
			case event := <-re.watchEvents:
			  if event.IsWrite() {
			    re.Reload(event.Name)
			  }
			case <-ctx.Done():
        close(re.watchEvents)  // ? needed ? impact
        return
			}
		}
	}()
	return re
}
func (re *Reloader) Remove(path string) {
	re.Lock()
	defer re.Unlock()
	c, ok := re.clients[path]
	if !ok {
		return
	}
	delete(re.clients, path)
	// closing c.send causes the client to terminate
	close(c.send)
}

func (re *Reloader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//FIXME: control access?
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	// upgrade this connection to a WebSocket connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	// add as a client
	c := &Client{ws: ws, send: make(chan []byte, msgBufSize)}
	re.Lock()
	re.clients[r.URL.Path] = c
	re.Unlock()
	// remove client when connection closed
	defer re.Remove(r.URL.Path)
	// wait for reads and writes
	go c.Write()
	c.Read()
}

func (re *Reloader) Reload(path string) error {
	re.RLock()
	c, ok := re.clients[path]
	re.Unlock()
	if !ok {
		return fmt.Errorf("not connected to %s", path)
	}
	select {
	case c.send <- msgReload: //send reload msg
	default: //cannot send
		re.Remove(path)
	}
	return nil
}

func injectReloadJS(f http.File, name string, mode fs.FileMode) (http.File, error) {
	// fmt.Println("injectReloadJS:", name, mode.String())
	if mode.IsDir() || !strings.HasPrefix(filepath.Ext(name), ".htm") {
		return nil, nil
	}
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var bodyTag = []byte("</body>")
	buf = bytes.Replace(buf, bodyTag, append(reloadJS, bodyTag...), 1)
	bf, err := NewFromReader(bytes.NewReader(buf), name, mode)
	if err != nil {
		return nil, err
	}
	_, err = bf.Seek(0, 0)

	if err != nil {
		return nil, err
	}
	err = f.Close()

	if err != nil {
		return nil, err
	}
	return bf, nil
}

var reloadJS = []byte(`
<script>var socket = new WebSocket("ws://localhost:5500/ws");
socket.onmessage = function(event) {
  console.log(event)
   switch(event.data) {
     case "reload":
       // 1000 = "Normal closure" and the second parameter is a
       // human-readable reason.
       socket.close(1000, "Reloading page after receiving reload");
       console.log("Reloading page after receiving build_complete");
       location.reload(true);
       break;

     default:
       console.log("recieved message:",event.data)
  }
}
</script>
`)
