package main

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

func Serve(basePaths map[string]string, streamablePath string, chunkSize int, port int) {
	http.HandleFunc("/", handler(basePaths, streamablePath, chunkSize))

	fmt.Println("Listening on port", port)
	http.ListenAndServe(fmt.Sprint(":", port), nil)
}

func handler(basePaths map[string]string, streamablePath string, chunkSize int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, flusherok := w.(http.Flusher)
		if !flusherok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}
		path := r.URL.Path
		pathParts := strings.Split(path, "/")
		fmt.Println("path prefix", pathParts[1])
		fmt.Println("full", r.FormValue("something"))
		realPath, realPathExists := basePaths[pathParts[1]]
		if realPath != "" && !realPathExists {
			http.Error(w, fmt.Sprint("Path ", path, " not found!"), http.StatusNotFound)
			return
		}
		fmt.Println("base path", realPath)
		fullPath := strings.Join(slices.Concat([]string{realPath}, pathParts[2:]), "/")
		fmt.Println("full path", fullPath)

		fmt.Println("method:", r.Method)
		if r.Method == http.MethodPost {
			post(w, r.Body, flusher, fullPath, chunkSize)
			return
		}
		if r.Method == http.MethodDelete {
			del(w, flusher, fullPath)
			return
		}

		get(w, flusher, fullPath, basePaths, path, streamablePath, chunkSize)
	}
}

func get(w http.ResponseWriter, flusher http.Flusher, fullPath string, basePaths map[string]string, path string, streamablePath string, chunkSize int) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")

	cDir := make(chan string)
	cFile := make(chan []byte)
	cErr := make(chan error)

	go Read(fullPath, basePaths, path, streamablePath, cErr, cDir, cFile, chunkSize)
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
				w.Write([]byte(fsItem))
			case chunk, chunkOk := <-cFile:
				if !chunkOk {
					cFileClosed = true
					break
				}
				w.Write(chunk)
				flusher.Flush()
			case err, errOk := <-cErr:
				if !errOk {
					cErrClosed = true
					break
				}
				fmt.Println("error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				flusher.Flush()
				return
			}
		}
		flusher.Flush()
	}(w, cErr, cDir, cFile)
}

func post(w http.ResponseWriter, body io.ReadCloser, flusher http.Flusher, fullPath string, chunkSize int) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	cErr := make(chan error)

	go Write(fullPath, body, cErr, chunkSize)
	func(w http.ResponseWriter, cErr <-chan error) {
		cErrClosed := false
		for !cErrClosed {
			select {
			case err, errOk := <-cErr:
				if !errOk {
					cErrClosed = true
					break
				}
				fmt.Println("error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				flusher.Flush()
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}(w, cErr)
}

func del(w http.ResponseWriter, flusher http.Flusher, fullPath string) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	cErr := make(chan error)

	go Delete(fullPath, cErr)
	func(w http.ResponseWriter, cErr <-chan error) {
		cErrClosed := false
		for !cErrClosed {
			select {
			case err, errOk := <-cErr:
				if !errOk {
					cErrClosed = true
					break
				}
				fmt.Println("error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				flusher.Flush()
				return
			}
		}
		w.Write([]byte("{}"))
	}(w, cErr)
}
