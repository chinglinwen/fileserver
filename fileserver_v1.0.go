/*
fileserver is Simple File Server through http
Can be used for yum repo, Or just share file through internet.

Create by Wen Zhenglin
Date: 2014-10-17
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {

	port := flag.String("port", "9000", "Port number.")
	path := flag.String("path", ".", "File server path.")
	version := flag.Bool("v", false, "Show version.")
	author := flag.Bool("author", false, "Show author.")

	flag.Parse()

	//Display version info.
	if *version {
		fmt.Println("fileserver version=1.0")
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

	//Usually it is no output, So no log.

	fileServer := http.FileServer(http.Dir(*path))

	// Simple static webserver:
	log.Fatal(http.ListenAndServe(":"+*port, fileServer))
}
