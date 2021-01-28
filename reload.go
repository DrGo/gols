package gols

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
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
	Addr        string
}

func NewReloader(ctx context.Context, addr string) *Reloader {
	re := &Reloader{
		clients:     make(map[string]*Client),
		toWatch:     make(chan string, 24),
		watchEvents: make(chan watcher.Event, 24),
		Addr:        addr,
	}
	//launch watcher
	go func() {
		err := watcher.Watch(ctx, re.toWatch, re.watchEvents)
		if err != nil {
			Logf("watcher.watch failed: %v\n", err)
		}
	}()
	// launch watch events handler
	go func() {
		for {
			select {
			case event := <-re.watchEvents:
				if event.IsWrite() {
				  Logln("recieved watch-event:" + event.String())
					re.Reload(event.Name)
				}
			case <-ctx.Done():
				close(re.watchEvents) // ? needed ? impact
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
		Logln(err)
		return
	}
	path := r.URL.Query().Get("id")
	if path == "" {
		Logln("invalid id websocket query")
		return
	}
	// add as a client
	c := &Client{ws: ws, send: make(chan []byte, msgBufSize)}
	re.Lock()
	re.clients[path] = c
	re.Unlock()
  Logln("relaod.ServeHTTP: added client for:" + path)
	// remove client when connection closed
	defer re.Remove(path)
	// wait for reads and writes
	go c.Write()
	c.Read()
}

func (re *Reloader) Reload(path string) error {
	// Logln("reloading " + path)
	re.RLock()
	c, ok := re.clients[path]
	defer re.RUnlock()
	if !ok {
		Logln("error relaoding: not watching this file: " + path)
		return fmt.Errorf("not connected to %s", path)
	}
	select {
	case c.send <- msgReload: //send reload msg
		Logln("sent reload to " + path)
	default: //cannot send
		re.Remove(path)
		Logln("failed to reload " + path)
	}
	return nil
}

func (re *Reloader) watchFiles(_ http.File, name string, mode fs.FileMode) (http.File, error) {
	// Logln("watchFiles:", name, mode.String())
	if mode.IsDir() || !strings.HasPrefix(filepath.Ext(name), ".htm") {
		return nil, nil
	}
	re.toWatch <- name
	return nil, nil
}

// func (re *Reloader) URLNameToPath(name string) string {
// 	return filepath.Clean(filepath.Join(re.Addr, name))
// }

// func (re *Reloader) PathToURLName(path string) string {
//   return strings.TrimPrefix(path, re.Addr)
// }

func (re *Reloader) injectReloadJS(f http.File, name string, mode fs.FileMode) (http.File, error) {
	// Logln("injectReloadJS:", name, mode.String())
	if mode.IsDir() || !strings.HasPrefix(filepath.Ext(name), ".htm") {
		return nil, nil
	}
  buf, err := io.ReadAll(f)
  if err != nil {
    return nil, fmt.Errorf("cannot inject file %s:%s", name, err)
  }
  url := append([]byte(re.Addr+"/ws?id="), []byte(url.QueryEscape(name))...)
	bf := NewFile(concatSlices([][]byte{buf, reloadJSOpen, url, reloadJSClose}), name, mode)
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

func concatSlices(slices [][]byte) []byte {
    var size int
    for _, s := range slices {
        size += len(s)
    }
    tmp := make([]byte,size)
    var i int
    for _, s := range slices {
        i += copy(tmp[i:], s)
    }
    return tmp
}

var (
	reloadJSOpen = []byte(`
<script>
function ws_connect() {
  var socket = new WebSocket("ws://`)

//see https://javascript.info/websocket
	reloadJSClose = []byte(`");
 socket.onclose = function(event) {
    console.log("Websocket connection failed or closed." + event.reason);
    socket = null;  //clean up last socket
    // Set an interval to continue trying to reconnect
    // setTimeout(function() {
    //   ws_connect();
    // }, 5000)
  }
 socket.onmessage = function(event) {
   switch(event.data) {
     case "reload":
       socket.close(1000, "Reloading page.."); //1000=normal closure
       console.log("Reloading page after receiving build_complete");
       location.reload(true);
       break;
     default:
       console.log("recieved message:",event.data) //debug only
  }
 }
}
ws_connect()
</script>
`)
)
