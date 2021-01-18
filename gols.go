// Package gols implements a simple HTTP server suitable for local development and testing
package gols

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"
)

type Config struct {
	Addr string
	// * @param watch {array} Paths to exclusively watch for changes
	// * @param ignore {array} Paths to ignore when watching files for changes
	// * @param ignorePattern {regexp} Ignore files by RegExp
	// * @param mount {array} Mount directories onto a route, e.g. [['/components', './node_modules']].
	// * @param logLevel {number} 0 = errors only, 1 = some, 2 = lots
	// * @param file {string} Path to the entry point file
	// * @param wait {number} Server will wait for all changes, before reloading
	// * @param htpasswd {string} Path to htpasswd file to enable HTTP Basic authentication
	Host          string
	Port          string
	Root          string
	CORS          bool
	Open          bool
	Watch         bool
	Ignore        string
	Quiet         bool
	Proxy         string
	AllowDotFiles bool
}

const (
	defaultPort = "5500"
	defaultHost = "localhost"
)

func ValidConfig(config *Config) (*Config, error) {
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
	return config, nil
}

type Server struct {
  config *Config
	mux *http.ServeMux
	srv *http.Server
}

func NewServer(config *Config) (*Server, error) {
	config, err := ValidConfig(config)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	fs := FS{fs: http.Dir(config.Root), entry: "index.html"}
	mux.Handle("/", NoCacheHandler(http.FileServer(fs)))
	s := &Server{
    config: config,
		mux: mux,
		srv: &http.Server{
			Addr:    config.Addr,
			Handler: mux,
			//FIXME: redirect server logs to app logs?
			//ErrorLog:     logger,
			// ReadTimeout:  5 * time.Second,
			// WriteTimeout: 10 * time.Second,
			// IdleTimeout:  15 * time.Second,
		},
	}
	return s,nil
}

func (s *Server) Finalize() {
}

// Serve Starts a live server with config
func Serve(config *Config) error {
	s, err := NewServer(config)
	if err!= nil {
	return err
  }
	return s.Serve()
}

func (s *Server) Serve() (err error) {
	if !s.config.Quiet {
		Logln("Serving folder:")
		Logln("   " + s.config.Root)
		Logln("Running at:")
		Logln("   http://" + s.config.Addr)
		Logln("Press ctrl+c to exit.")
	}
	// gracefully handle keyboard interruptions ctrl+c etc
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	done := make(chan interface{}) // chan to ensure that we do not exist before s.Shutdown() is done
	go func() {
		<-stop // blocks until it receives an interrupt signal
		Logf("\nserver stopping...\n")
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
		go func() {
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
			_ = exec.Command(cmd, append(args, "http://"+s.config.Addr)...).Start()
		}()
	}
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server on %s: %v", s.config.Addr, err)
	}
	<-done //block until shutdown is complete
	Logf("\nserver stopped...\n")
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

// func getIPAddr() {
// 	if addrs, err := net.InterfaceAddrs(); err == nil {
// 		for _, a := range addrs {
// 			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
// 				Logln("   http://" + ipnet.IP.String() + ":" + config.Port)
// 			}
// 		}
// 	}
// }
