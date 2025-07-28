package main

import (
	"fmt"
	"os"
	"rnas/streaming"
	"strings"
)

func Delete(fullPath string, virtualPath string, streamablePath string, cErr chan error) {
	fmt.Println("hit delete")
	defer close(cErr)

	file, createErr := os.Open(fullPath)
	if createErr != nil {
		cErr <- createErr
		return
	}
	if file == nil {
		cErr <- fmt.Errorf("File at %s does not exist", fullPath)
	}
	removeErr := os.Remove(fullPath)
	if removeErr != nil {
		cErr <- removeErr
	}

	_, sanitisedFileName, virtualPathPrefix := streaming.GetStreamStrings(virtualPath)
	streamDir, streamDirErr := streaming.GetStreamablePath(sanitisedFileName, virtualPathPrefix, streamablePath)
	if streamDirErr != nil {
		cErr <- fmt.Errorf("Error reading file or directory: %s", streamDirErr.Error())
		return
	}
	streamFiles, dirErr := os.ReadDir(*streamDir)
	if dirErr != nil {
		cErr <- fmt.Errorf("Error reading streamable path %s for deletion: %s", *streamDir, dirErr.Error())
		return
	}
	if streamDir == nil {
		fmt.Println("File should now be deleted at ", fullPath)
		return
	}
	for _, f := range streamFiles {
		if !strings.HasPrefix(f.Name(), sanitisedFileName) {
			continue
		}
		removeErr := os.Remove(*streamDir + "/" + f.Name())
		if removeErr != nil {
			cErr <- fmt.Errorf("Error deleting streaming file for deleted file %s at %s: %s", f.Name(), *streamDir, removeErr.Error())
			return
		}
	}

	fmt.Println("Streaming file should now be deleted at ", fullPath)
}
