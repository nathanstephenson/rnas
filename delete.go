package main

import (
	"fmt"
	"os"
)

func Delete(fullPath string, cErr chan error) {
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

	fmt.Println("File should now be deleted at ", fullPath)
}
