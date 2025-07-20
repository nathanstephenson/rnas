package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type File struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Bytes []byte `json:"bytes"`
}

func Write(fullPath string, body io.ReadCloser, cErr chan error, chunkSize int) {
	fmt.Println("hit write")
	defer close(cErr)

	bodyBytes, readErr := io.ReadAll(body)
	if readErr != nil {
		cErr <- readErr
		return
	}
	body.Close()
	fmt.Println("read body")
	files := []File{}
	jsonErr := json.Unmarshal(bodyBytes, &files)
	if jsonErr != nil {
		cErr <- jsonErr
		return
	}
	fmt.Println("got", len(files), "files:")

	dir, dirErr := os.ReadDir(fullPath)
	if dirErr != nil {
		cErr <- dirErr
		return
	}
	for _, f := range files {
		fileName := f.Name
		fmt.Println(fileName)
		for _, dirEntry := range dir {
			if dirEntry.Name() == fileName {
				err := fmt.Errorf("File with name %s already exists in directory", fileName)
				cErr <- err
				return
			}
		}

		file, createErr := os.Create(fullPath + fileName)
		if createErr != nil {
			cErr <- createErr
			return
		}
		defer file.Close()
		_, writeErr := file.Write(f.Bytes)
		if writeErr != nil {
			cErr <- writeErr
			return
		}
	}

	fmt.Println("File should now be available at ", fullPath)
}
