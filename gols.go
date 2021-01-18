package gols

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

type Config struct {
	Addr   string
	Host   string
	Port   string
	Root   string
	CORS   bool
	Open   bool
	Watch  bool
	Ignore string
	Quiet  bool
	Proxy  string
}

var config Config

const (
	defaultPort = "5500"
	defaultHost = "localhost"
)

	// file: file,
	// open: false,
	// https: https,
	// ignore: ignoreFiles,
	// disableGlobbing: true,
	// proxy: proxy,
	// cors: true,
	// wait: Config.getWait || 100,

	// * Start a live server with parameters given as an object
	// * @param watch {array} Paths to exclusively watch for changes
	// * @param ignore {array} Paths to ignore when watching files for changes
	// * @param ignorePattern {regexp} Ignore files by RegExp
	// * @param mount {array} Mount directories onto a route, e.g. [['/components', './node_modules']].
	// * @param logLevel {number} 0 = errors only, 1 = some, 2 = lots
	// * @param file {string} Path to the entry point file
	// * @param wait {number} Server will wait for all changes, before reloading
	// * @param htpasswd {string} Path to htpasswd file to enable HTTP Basic authentication

func Serve(config *Config) error {
	if config.Root == "" {
		if pwd, err := os.Getwd(); err != nil {
			return fmt.Errorf("cannot determine root dir: %v", err)
		} else {
			config.Root = pwd
		}
	}
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

	NoCacheHandler := func(h http.Handler) http.HandlerFunc {
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
	addr := config.Host + ":" + config.Port
	if config.Open {
		go func() {
			time.Sleep(time.Second)

			log.Println("Serving folder:")
			log.Println("   " + config.Root)
			log.Println("Running at:")
			log.Println("   http://" + addr)
      log.Println("Press ctrl+c to exit.")
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
		}()
	}
  fs := FS{fs: http.Dir(config.Root),entry: "index.html"}
	handler := NoCacheHandler(http.FileServer(fs))
	// if runtime.GOOS == "darwin" {
	// 	addr = "localhost:" + config.Port
	// }
	if err := http.ListenAndServe(addr, handler); err != nil {
		return fmt.Errorf("failed to start server on %s: %v", addr, err)
	}
	return nil
}

func getIPAddr() {
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				log.Println("   http://" + ipnet.IP.String() + ":" + config.Port)
			}
		}
	}
}
