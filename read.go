package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

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

	splitPath := strings.Split(virtualPath, "/")
	fileName := splitPath[len(splitPath)-1]
	virtualPathPrefix := strings.Join(splitPath[:len(splitPath)-1], "/")
	streamDir, streamDirErr := getStreamablePath(fileName, virtualPathPrefix, streamablePath)
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
	}

	defer file.Close()
	if err != nil {
		return fmt.Errorf("Error reading file 2: %s", err.Error())
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

func runFfmpeg(fileName string, path string, outputPath string) error {
	filterComplex := "[0:v]split=2[v1][v2]; [v1]scale=w=1920:h=1080[v1out]; [v2]scale=w=1280:h=720[v2out]"
	mapV1 := []string{"-map", "[v1out]", "-c:v:0", "libx264", "-b:v:0", "5000k", "-maxrate:v:0", "5350k", "-bufsize:v:0", "7500k"}
	mapV2 := []string{"-map", "[v2out]", "-c:v:1", "libx264", "-b:v:1", "2000k", "-maxrate:v:1", "2996k", "-bufsize:v:1", "4200k"}
	m1 := []string{"-map", "a:0", "-c:a", "aac", "-b:a:0", "192k", "-ac", "2"}
	m2 := []string{"-map", "a:0", "-c:a", "aac", "-b:a:1", "128k", "-ac", "2"}
	args := slices.Concat([]string{"-i", path, "-filter_complex", filterComplex}, mapV1, mapV2, m1, m2, []string{"-hls_list_size", "0", "-f", "hls", "-hls_time", "10", "-hls_playlist_type", "vod", "-hls_flags", "independent_segments", "-hls_segment_type", "mpegts", "-hls_segment_filename", fmt.Sprintf("%s%s%s", outputPath, fileName, "%v-%03d.ts"), "-master_pl_name", fmt.Sprintf("%s.%s", fileName, "m3u8"), "-var_stream_map", "v:0,a:0 v:1,a:1", fmt.Sprintf("%s%s%s", outputPath, fileName, "%v-playlist.m3u8")})
	command := exec.Command("ffmpeg", args...)
	fmt.Println(fmt.Sprintf("ffmpeg input: %s", command.String()))
	stdout, cmdErr := command.Output()
	if cmdErr != nil {
		return fmt.Errorf("Error running ffmpeg: %s", cmdErr.Error())
	}
	fmt.Println(fmt.Sprintf("ffmpeg output: %s", string(stdout)))
	return nil
}

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
	dir, err := os.ReadDir(outputPath)
	if err != nil {
		fmt.Println("not found, creating dir", outputPath)
		mkdirErr := os.MkdirAll(outputPath, 0777)
		if mkdirErr != nil {
			return nil, nil, mkdirErr
		}
	}
	fmt.Println("trying to open file, ", outputFilePath)
	file, err := os.Open(outputFilePath)
	isLoadingFile := false
	for _, subdir := range dir {
		if strings.HasPrefix(subdir.Name(), sanitisedFileName) {
			isLoadingFile = true
		}
	}
	if err != nil && !isLoadingFile {
		ffmpegErr := runFfmpeg(sanitisedFileName, path, outputPath)
		if ffmpegErr != nil {
			return nil, nil, ffmpegErr
		}
	}
	for {
		if !isLoadingFile {
			break
		}
		fmt.Println("loading file, waiting", outputPath, outputFileName)
		dir, err = os.ReadDir(outputPath)
		if err != nil {
			return nil, nil, err
		}
		for _, subdir := range dir {
			if !isLoadingFile {
				break
			}
			if strings.HasPrefix(subdir.Name(), sanitisedFileName) {
				isLoadingFile = true
			}
			if subdir.Name() == outputFileName {
				isLoadingFile = false
			}
		}
	}
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

func getStreamablePath(fileName string, pathPrefix string, streamablePath string) (*string, error) {
	dir, err := os.ReadDir(streamablePath)
	if err != nil {
		return nil, err
	}
	for _, subdir := range dir {
		dirname := subdir.Name()
		isDir := subdir.IsDir()
		if isDir && strings.Contains(pathPrefix, dirname) {
			fmt.Println("going deeper:", dirname)
			found, findErr := getStreamablePath(fileName, pathPrefix, fmt.Sprintf("%s/%s", streamablePath, dirname))
			if findErr != nil {
				return nil, findErr
			}
			if found != nil {
				return found, nil
			}
		}
		fmt.Println("dirname:", dirname)
		if !isDir && fileName == dirname {
			fmt.Println("found", fileName, dirname)
			return &streamablePath, nil
		}
	}
	return nil, nil
}
