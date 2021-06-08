// Package gols implements a simple HTTP server suitable for local development and testing
package gols

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"
)

// * @param watch {array} Paths to exclusively watch for changes
// * @param ignore {array} Paths to ignore when watching files for changes
// * @param ignorePattern {regexp} Ignore files by RegExp
// * @param mount {array} Mount directories onto a route, e.g. [['/components', './node_modules']].
// * @param wait {number} Server will wait for all changes, before reloading
// * @param file {string} Path to the entry point file
// * @param htpasswd {string} Path to htpasswd file to enable HTTP Basic authentication

// Config used to configure the server
// A Server defines parameters for running an HTTP server.
// The zero value for Server is a valid configuration.
type Config struct {
	// Addr optionally specifies the TCP address for the server to listen on,
	// in the form "host:port". If empty, the values of Host:Port are used.
	// The service names are defined in RFC 6335 and assigned by IANA.
	Addr string

	// Host optionally specifies the host name. Default "localhost."
	Host string

	// Port optionally specifies the port number. Default 5500.
	Port string

	Root          string
	CORS          bool //TODO
	Open          bool
	Ignore        string //TODO:?
	Quiet         bool
	Proxy         string //TODO:
	AllowDotFiles bool
	// if true, changes to FS contents cause the browser to reload the changed files
	LiveRelood bool

	AllowCaching bool

	/// ReadTimeout is the maximum duration for reading the entire
	// request, including the body.
	//
	// Because ReadTimeout does not let Handlers make per-request
	// decisions on each request body's acceptable deadline or
	// upload rate, most users will prefer to use
	// ReadHeaderTimeout. It is valid to use them both.
	// Default is 5 secs.
	ReadTimeout time.Duration

	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers. The connection's read deadline is reset
	// after reading the headers and the Handler can decide what
	// is considered too slow for the body. If ReadHeaderTimeout
	// is zero, the value of ReadTimeout is used. If both are
	// zero, default of 5 secs is used.
	ReadHeaderTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out
	// writes of the response. It is reset whenever a new
	// request's header is read. Like ReadTimeout, it does not
	// let Handlers make decisions on a per-request basis.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alives are enabled. If IdleTimeout
	// is zero, it defaults to 15 secs.
	IdleTimeout time.Duration

	// MaxHeaderBytes controls the maximum number of bytes the
	// server will read parsing the request header's keys and
	// values, including the request line. It does not limit the
	// size of the request body.
	// If zero, DefaultMaxHeaderBytes is used.
	MaxHeaderBytes int

	// ErrorLog specifies an optional logger for errors accepting
	// connections, unexpected behavior from handlers, and
	// underlying FileSystem errors.
	// If nil, logging is done via the log package's standard logger.
	ErrorLog *log.Logger
}

const (
	defaultPort = "5500"
	defaultHost = "localhost"
)

func validateConfig(config *Config) (*Config, error) {
	if config == nil {
		config = &Config{}
	}
	if config.Root == "" {
		if pwd, err := os.Getwd(); err != nil {
			return nil, fmt.Errorf("cannot determine root dir: %v", err)
		} else {
			config.Root = pwd
		}
	}
	// if runtime.GOOS == "darwin" {
	// 	addr = "localhost:" + config.Port
	// }
	if config.Addr == "" {
		if config.Host == "" {
			config.Host = defaultHost
		}
		if config.Port == "" {
			config.Port = defaultPort
		}
		config.Addr = config.Host + ":" + config.Port
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 5 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Second
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 15 * time.Second
	}
	if config.ErrorLog == nil {
		config.ErrorLog = log.New(os.Stderr, "gols:", log.LstdFlags)
	}
	return config, nil
}

type Server struct {
	config *Config
	mux    *http.ServeMux
	srv    *http.Server
}

func NewServer(ctx context.Context, config *Config) (*Server, error) {
	config, err := validateConfig(config)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	fs := FS{
		// FIXME: allow fs to be passed in config
		fs:            http.Dir(config.Root),
		entry:         "index.html",
		root:          config.Root,
		AllowDotFiles: config.AllowDotFiles,
	}
	if config.LiveRelood {
		reloader := NewReloader(ctx, config.Addr)
		fs.BeforeServing = reloader.injectReloadJS
		fs.AfterServing = reloader.watchFiles
		mux.Handle("/ws", reloader)
	}
	if config.AllowCaching {
		mux.Handle("/", http.FileServer(fs))
	} else {
		mux.Handle("/", NoCacheHandler(http.FileServer(fs)))
	}
	s := &Server{
		config: config,
		mux:    mux,
		srv: &http.Server{
			Addr:    config.Addr,
			Handler: mux,
			//FIXME: redirect server logs to app logs?
			//ErrorLog:     logger,
			ReadTimeout:    config.ReadTimeout,
			WriteTimeout:   config.WriteTimeout,
			IdleTimeout:    config.IdleTimeout,
			MaxHeaderBytes: config.MaxHeaderBytes,
		},
	}
	return s, nil
}

func (s *Server) Finalize() {
}

// Serve Starts a live server with config
func Serve(ctx context.Context, config *Config) error {
	s, err := NewServer(ctx, config)
	if err != nil {
		return err
	}
	return s.Serve(ctx)
}

func (s *Server) Serve(ctx context.Context) (err error) {
	if !s.config.Quiet {
    Logf("Serving %s at %s\n", s.config.Root, s.config.Addr)
		Logln("Press ctrl+c to exit.")
	}
	// gracefully handle keyboard interruptions ctrl+c etc
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	done := make(chan interface{}) // chan to ensure that we do not exist before s.Shutdown() is done

	go func() {
		select { //block until interrupted or cancelled
		case <-stop: // we received an interrupt signal
			Logf("\nserver interrupted. stopping...\n")
		case <-ctx.Done():
			Logf("\nserver cancelled. stopping...\n")
		}
		// allow time for all goroutines to finish
		ctxWait, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.srv.SetKeepAlivesEnabled(false) //disable keepAlive
		if err = s.srv.Shutdown(ctxWait); err != nil {
			err = fmt.Errorf("server failed to shutdown:%v", err)
		}
		close(done)
	}()
	// open in browser if requested
	if s.config.Open {
		go broswe(s.config.Addr)
	}
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server on %s: %v", s.config.Addr, err)
	}
	<-done //block until shutdown is complete
	Logf("\nserver stopped\n")
	return nil
}

// prevent caching when reloading during dev work
// http://stackoverflow.com/questions/33880343/go-webserver-dont-cache-files-using-timestamp
var epoch = time.Unix(0, 0).Format(time.RFC1123)
var noCacheHeaders = map[string]string{
	"Expires":         epoch,
	"Cache-Control":   "no-cache, private, max-age=0",
	"Pragma":          "no-cache",
	"X-Accel-Expires": "0",
}
var etagHeaders = []string{
	"ETag",
	"If-Modified-Since",
	"If-Match",
	"If-None-Match",
	"If-Range",
	"If-Unmodified-Since",
}

func NoCacheHandler(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, v := range etagHeaders {
			if r.Header.Get(v) != "" {
				r.Header.Del(v)
			}
		}
		for k, v := range noCacheHeaders {
			w.Header().Set(k, v)
		}
		h.ServeHTTP(w, r)
	}
}

func broswe(addr string) {
	time.Sleep(time.Second)
	// https://stackoverflow.com/questions/39320371/how-start-web-server-to-open-page-in-browser-in-golang
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	_ = exec.Command(cmd, append(args, "http://"+addr)...).Start()
}

// func getIPAddr() {
// 	if addrs, err := net.InterfaceAddrs(); err == nil {
// 		for _, a := range addrs {
// 			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
// 				Logln("   http://" + ipnet.IP.String() + ":" + config.Port)
// 			}
// 		}
// 	}
// }
