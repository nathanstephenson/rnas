package main

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
)

func Serve(basePaths map[string]string, chunkSize int, port int) {
	http.HandleFunc("/", handler(basePaths, chunkSize))

	fmt.Println("Listening on port", port)
	http.ListenAndServe(fmt.Sprint(":", port), nil)
}

func handler(basePaths map[string]string, chunkSize int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, flusherok := w.(http.Flusher)
		if !flusherok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}
		path := r.URL.Path
		pathParts := strings.Split(path, "/")
		fmt.Println("path prefix", pathParts[1])
		realPath, realPathExists := basePaths[pathParts[1]]
		if !realPathExists {
			http.Error(w, fmt.Sprint("Path ", path, " not found!"), http.StatusNotFound)
			return
		}
		fmt.Println("base path", realPath)
		fullPath := strings.Join(slices.Concat([]string{realPath}, pathParts[2:]), "/")
		fmt.Println("full path", fullPath)

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)

		cDir := make(chan string)
		cFile := make(chan []byte)
		cErr := make(chan error)

		go Read(fullPath, cErr, cDir, cFile, chunkSize)
		func(w http.ResponseWriter, cErr <-chan error, cDir <-chan string, cFile <-chan []byte) {
			cFileClosed := false
			cDirClosed := false
			cErrClosed := false
			for !cFileClosed || !cDirClosed || !cErrClosed {
				select {
				case fsItem, fsItemOk := <-cDir:
					if !fsItemOk {
						cDirClosed = true
						break
					}
					fmt.Println("fs item", fsItem)
					w.Write([]byte(fsItem))
				case chunk, chunkOk := <-cFile:
					if !chunkOk {
						cFileClosed = true
						break
					}
					fmt.Println("chunk", chunk)
					w.Write(chunk)
					flusher.Flush()
				case err, errOk := <-cErr:
					if !errOk {
						cErrClosed = true
						break
					}
					fmt.Println("error", err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}(w, cErr, cDir, cFile)
	}
}
