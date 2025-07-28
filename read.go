package main

import (
	"encoding/json"
	"fmt"
	"os"
	"rnas/streaming"
	"strings"
	"sync"

	"github.com/gabriel-vasile/mimetype"
)

func Read(path string, basePaths map[string]string, virtualPath string, streamablePath string, cErr chan<- error, cDir chan<- string, cFile chan<- []byte, chunkSize int) {
	defer close(cErr)

	if path == "" {
		close(cFile)
		baseErr := readBase(basePaths, cDir)
		if baseErr != nil {
			cErr <- baseErr
		}
		return
	}

	fileName, _, virtualPathPrefix := streaming.GetStreamStrings(virtualPath)
	streamDir, streamDirErr := streaming.GetStreamablePath(fileName, virtualPathPrefix, streamablePath)
	if streamDirErr != nil {
		cErr <- fmt.Errorf("Error reading file or directory: %s", streamDirErr.Error())
		return
	}
	if streamDir != nil {
		close(cDir)
		fmt.Println("is streaming file!", path, virtualPath)
		streamFilePath := fmt.Sprintf("%s/%s", *streamDir, fileName)
		fileErr := readFile(streamFilePath, virtualPath, streamablePath, cFile, chunkSize)
		if fileErr != nil {
			cErr <- fileErr
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
	fileErr := readFile(path, virtualPath, streamablePath, cFile, chunkSize)
	if fileErr != nil {
		cErr <- fileErr
	}
	return
}

type DirInfo struct {
	Type  string `json:"type"` // should always be "directory"
	Name  string `json:"name"`
	Count int    `json:"count"`
}
type FileInfo struct {
	Type     string `json:"type"` // should always be "file"
	MimeType string `json:"mime"`
	Name     string `json:"name"`
	Size     int    `json:"size"`
	Modified int    `json:"modified"`
}

func readFile(path string, virtualPath string, streamablePath string, c chan<- []byte, chunkSize int) error {
	defer close(c)

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Error reading file: %s", err.Error())
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Error reading file: %s", err.Error())
	}

	mime, mimeErr := mimetype.DetectFile(path)
	if mimeErr != nil {
		return mimeErr
	}

	if strings.HasPrefix(mime.String(), "video/") {
		fmt.Println(fmt.Sprintf("this is a video: %v (%s)", mime.String(), path))
		file, fileInfo, err = getStreamFile(path, virtualPath, streamablePath)
		if err != nil {
			return fmt.Errorf("Error reading streaming file: %s", err.Error())
		}
	}
	defer file.Close()

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

	if len(files) == 0 {
		c <- "[]"
		return nil
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
				return fmt.Errorf("Error marshalling directory info: %s", err.Error())
			}
			c <- string(s)
		} else { // if file
			mime, mimeErr := mimetype.DetectFile(fmt.Sprintf("%v/%v", path, file.Name()))
			if mimeErr != nil {
				return mimeErr
			}
			s, err := json.Marshal(FileInfo{Type: "file", MimeType: mime.String(), Name: file.Name(), Size: int(file.Size()), Modified: int(file.ModTime().Unix())})
			if err != nil {
				return fmt.Errorf("Error marshalling file info: %s", err.Error())
			}
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
		return nil, fmt.Errorf("Error getting files from directory %s: %s", path, err.Error())
	}
	return &DirInfo{Type: "directory", Name: name, Count: len(subFiles)}, nil
}

var ffmpegRunsLock = sync.RWMutex{}
var ffmpegRuns = map[string]bool{}

func getStreamFile(path string, virtualPath string, streamablePath string) (*os.File, os.FileInfo, error) {
	parts := strings.Split(virtualPath, "/")
	fileName := parts[len(parts)-1]
	if fileName == "" {
		return nil, nil, fmt.Errorf("filename could not be found at virtual path %s", virtualPath)
	}
	sanitisedFileName := strings.ReplaceAll(fileName, ".", "-")
	outputPath := fmt.Sprintf("%s%s/", streamablePath, strings.Join(parts[:len(parts)-1], "/"))
	outputFileName := fmt.Sprintf("%s.%s", sanitisedFileName, "m3u8")
	outputFilePath := fmt.Sprintf("%s%s", outputPath, outputFileName)
	ffmpegRunsLock.Lock()
	dir, err := os.ReadDir(outputPath)
	if err != nil {
		fmt.Println("not found, creating dir", outputPath)
		mkdirErr := os.MkdirAll(outputPath, 0777)
		if mkdirErr != nil {
			return nil, nil, mkdirErr
		}
	}
	fmt.Println("trying to open file, ", outputFilePath, "in", outputPath, " - ", len(dir), "files")
	file, err := os.Open(outputFilePath)
	isLoadingFile, hasLoadedFile := ffmpegRuns[outputFilePath]
	fileLoading := isLoadingFile && hasLoadedFile
	for _, subdir := range dir {
		fmt.Println("looking at", subdir.Name(), "for", sanitisedFileName)
		if strings.HasPrefix(subdir.Name(), sanitisedFileName) {
			fmt.Println("found loading file")
			fileLoading = true
			break
		}
	}
	if err != nil && !fileLoading {
		ffmpegRuns[outputFilePath] = true
		ffmpegErr := streaming.RunFfmpeg(sanitisedFileName, path, outputPath)
		if ffmpegErr != nil {
			delete(ffmpegRuns, outputFilePath)
			return nil, nil, ffmpegErr
		}
	}
	for {
		isLoadingFile, hasLoadedFile = ffmpegRuns[outputFilePath]
		fileLoading = isLoadingFile && hasLoadedFile
		if !fileLoading {
			break
		}
		dir, err = os.ReadDir(outputPath)
		if err != nil {
			return nil, nil, err
		}
		for _, subdir := range dir {
			if !fileLoading {
				break
			}
			if strings.HasPrefix(subdir.Name(), sanitisedFileName) {
				fileLoading = true
			}
			if subdir.Name() == outputFileName {
				ffmpegRuns[outputFilePath] = false
			}
		}
	}
	ffmpegRunsLock.Unlock()
	file, err = os.Open(outputFilePath)
	if err != nil {
		return nil, nil, err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	return file, fileInfo, nil
}
