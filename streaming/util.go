package streaming

import (
	"fmt"
	"os"
	"strings"
)

func GetStreamStrings(virtualPath string) (fileName string, sanitisedFileName string, virtualPathPrefix string) {
	splitPath := strings.Split(virtualPath, "/")
	virtualPathPrefix = strings.Join(splitPath[:len(splitPath)-1], "/")
	fileName = splitPath[len(splitPath)-1]
	sanitisedFileName = strings.ReplaceAll(fileName, ".", "-")
	return
}

func GetStreamablePath(fileName string, pathPrefix string, streamablePath string) (*string, error) {
	fmt.Println("filename:", fileName)
	dir, err := os.ReadDir(streamablePath)
	if err != nil {
		return nil, err
	}
	for _, subdir := range dir {
		dirname := subdir.Name()
		isDir := subdir.IsDir()
		if isDir && strings.Contains(pathPrefix, dirname) {
			fmt.Println("going deeper:", dirname)
			found, findErr := GetStreamablePath(fileName, pathPrefix, fmt.Sprintf("%s/%s", streamablePath, dirname))
			if findErr != nil {
				return nil, findErr
			}
			if found != nil {
				return found, nil
			}
		}
		fmt.Println("dirname:", dirname)
		if !isDir && (fileName == dirname || strings.HasPrefix(dirname, fileName)) {
			fmt.Println("found", fileName, dirname)
			return &streamablePath, nil
		}
	}
	return nil, nil
}
