package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	portStr, portexists := os.LookupEnv("PORT")
	if !portexists {
		log.Fatal("Could not find PORT env var")
	}
	port, porterr := strconv.Atoi(portStr)
	if porterr != nil {
		log.Fatal("Error converting PORT env var to int", porterr.Error())
	}

	paths, patherr := getPaths(1, map[string]string{})
	if patherr != nil {
		log.Fatal("Error getting path data from env vars", patherr.Error())
	}

	chunkSizeStr, hasChunkSize := os.LookupEnv("CHUNK_SIZE")
	if !hasChunkSize {
		chunkSizeStr = "2048"
	}
	chunkSize, chunkSizeErr := strconv.Atoi(chunkSizeStr)
	if chunkSizeErr != nil {
		log.Fatal("Error converting MAX_FILE_SIZE_MB env var to int", chunkSizeErr.Error())
	}

	fmt.Println("Port:", port, "Paths:", paths)
	Serve(paths, chunkSize, port)
}

func getPaths(pathNumber int, paths map[string]string) (map[string]string, error) {
	varname := fmt.Sprint("PATH_", pathNumber)
	path, pathexists := os.LookupEnv(varname)
	if !pathexists {
		return paths, nil
	}
	pathname, pathnameexists := os.LookupEnv(varname + "_NAME")
	if !pathnameexists {
		return nil, errors.New(fmt.Sprint("PATH_%d exists but PATH_%d_NAME does not", pathNumber, pathNumber))
	}
	paths[pathname] = path
	return getPaths(pathNumber+1, paths)
}
