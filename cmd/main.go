package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"github.com/drgo/gols"
)

//TODO: add cpu/mem profile expvar

var config gols.Config

const (
	defaultPort = "5500"
	defaultHost = "localhost"
)

func init() {
	flag.StringVar(&config.Port, "port", defaultPort, "listening port (0 for random; default"+defaultPort+")")
	flag.BoolVar(&config.Open, "open", true, "open first page in a browser automatically")
	flag.StringVar(&config.Host, "host", defaultHost, "Address to bind to (default:"+defaultHost+")")
	flag.StringVar(&config.Root, "root", "", "Path to root directory (default: cwd)")

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
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	// // profilign support
	// if *cpuprofile != "" {
	// 	f, err := os.Create(*cpuprofile)
	// 	if err != nil {
	// 		errors.Fatal(err)
	// 	}
	// 	pprof.StartCPUProfile(f)
	// 	defer pprof.StopCPUProfile()
	// }
  fmt.Println(config)
  if err :=gols.Serve(&config); err!= nil {
    fmt.Printf("%s:%v", os.Args[0], err)
  }
}

