module github.com/drgo/gols

go 1.17

require (
	github.com/drgo/core v0.1.5
	github.com/gorilla/websocket v1.4.2
	github.com/mattetti/filebuffer v1.0.1
)

require (
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	golang.org/x/sys v0.0.0-20191005200804-aed5e4c7ecf9 // indirect
)

replace github.com/drgo/core => ../core
