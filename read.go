package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func Read(path string, basePaths map[string]string, cErr chan<- error, cDir chan<- string, cFile chan<- []byte, chunkSize int) {
	defer close(cErr)

	if path == "" {
		close(cFile)
		baseErr := readBase(basePaths, cDir)
		if baseErr != nil {
			cErr <- baseErr
		}
		return
	}

	info, fsErr := os.Stat(path)
	if fsErr != nil {
		cErr <- fmt.Errorf("Error reading file or directory: %s", fsErr.Error())
		return
	}
	if info.IsDir() {
		close(cFile)
		dirErr := readDir(path, cDir)
		if dirErr != nil {
			cErr <- dirErr
		}
		return
	}
	close(cDir)
	fileErr := readFile(path, cFile, chunkSize)
	if fileErr != nil {
		cErr <- fileErr
	}
	return
}

type DirInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
type FileInfo struct {
	Name     string `json:"name"`
	Size     int    `json:"size"`
	Modified int    `json:"modified"`
}

func readFile(path string, c chan<- []byte, chunkSize int) error {
	defer close(c)

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Error reading file: %s", err.Error())
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Error reading file: %s", err.Error())
	}

	for offset := int64(0); offset < fileInfo.Size(); offset += int64(chunkSize) {
		realChunkSize := min(chunkSize, int(fileInfo.Size()-offset))
		fileBytes := make([]byte, realChunkSize)
		_, err = file.ReadAt(fileBytes, offset)
		if err != nil {
			return fmt.Errorf("Error reading file: %s", err.Error())
		}

		c <- fileBytes
	}
	return nil
}

func readDir(path string, c chan<- string) error {
	defer close(c)

	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Error reading directory: %s", err.Error())
	}
	defer dir.Close()

	files, err := dir.Readdir(0)
	if err != nil {
		return fmt.Errorf("Error reading directory: %s", err.Error())
	}

	for idx, file := range files {
		if idx == 0 {
			c <- "["
		}

		if file.IsDir() {
			separator := "/"
			if strings.HasSuffix(path, "/") {
				separator = ""
			}
			subPath := path + separator + file.Name()
			dirInfo, dirErr := getDirInfo(file.Name(), subPath)
			if dirErr != nil {
				return dirErr
			}
			s, err := json.Marshal(dirInfo)
			if err != nil {
				fmt.Println(fmt.Errorf("Error marshalling directory info: %s", err.Error()))
				continue
			}
			fmt.Println("dir", s)
			c <- string(s)
		} else { // if file
			s, err := json.Marshal(FileInfo{Name: file.Name(), Size: int(file.Size()), Modified: int(file.ModTime().Unix())})
			if err != nil {
				fmt.Println(fmt.Errorf("Error marshalling file info: %s", err.Error()))
				continue
			}
			fmt.Println("file", s)
			c <- string(s)
		}

		if idx == len(files)-1 {
			c <- "]"
		} else {
			c <- ","
		}
	}
	return nil
}

func readBase(basePaths map[string]string, c chan<- string) error {
	defer close(c)

	idx := 0
	for name, path := range basePaths {
		if idx == 0 {
			c <- "["
		}
		dirInfo, dirErr := getDirInfo(name, path)
		if dirErr != nil {
			return dirErr
		}
		s, err := json.Marshal(dirInfo)
		if err != nil {
			fmt.Println(fmt.Errorf("Error marshalling directory info: %s", err.Error()))
			continue
		}
		fmt.Println("dir", s)
		c <- string(s)

		if idx == len(basePaths)-1 {
			c <- "]"
		} else {
			c <- ","
		}
		idx++
	}
	return nil
}

func getDirInfo(name string, path string) (*DirInfo, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Error reading directory %s: %s", path, err.Error())
	}
	defer dir.Close()
	subFiles, err := dir.Readdir(0)
	if err != nil {
		return nil, fmt.Errorf("Error reading directory: %s", err.Error())
	}
	return &DirInfo{Name: name, Count: len(subFiles)}, nil
}
