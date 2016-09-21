/*
fileserver is Simple File Server through http
Can be used for yum repo, Or just share file through internet.

Create by Wen Zhenglin
Date: 2014-10-17

v2: add request log output
Date: 2016-9-21
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/natefinch/lumberjack"
)

func main() {

	port := flag.String("port", "9000", "Port number.")
	path := flag.String("path", ".", "File server path.")
	version := flag.Bool("v", false, "Show version.")
	author := flag.Bool("author", false, "Show author.")

	logFile := flag.String("logfile", "fs.log", "log filename and path")
	logMaxSize := flag.Int("logmaxsize", 500, "log max size(megabytes)")
	logMaxAge := flag.Int("logmaxage", 28, "log max age (days)")
	logMaxBackups := flag.Int("logmaxbackups", 3, "log max backups number")

	flag.Parse()

	//Display version info.
	if *version {
		fmt.Println("fileserver version=2.0, 2016-9-21")
		os.Exit(0)
	}

	//Display author info.
	if *author {
		fmt.Println("Author is Wen Zhenglin")
		os.Exit(0)
	}

	if len(os.Args) == 2 {
		argsPath := os.Args[1]
		if argsPath != "" {
			fmt.Println(&argsPath)
			path = &argsPath
		}
	}

	log.SetOutput(&lumberjack.Logger{
		Filename:   *logFile,
		MaxSize:    *logMaxSize, // megabytes
		MaxBackups: *logMaxBackups,
		MaxAge:     *logMaxAge, //days
	})

	fileServer := http.FileServer(http.Dir(*path))

	log.Println("Starting service at port:", *port)

	// Simple static webserver:
	log.Fatal(http.ListenAndServe(":"+*port, logHandler(fileServer.ServeHTTP)))
}

func logHandler(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := strings.Split(r.RemoteAddr, ":")[0]
		log.Println(ip, r.RequestURI)
		fn(w, r)
	}
}
