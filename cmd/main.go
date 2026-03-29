package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	githttp "github.com/qoppa-tech/toy-gitfed/internal/api/http"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <repos-dir> [port]\n", os.Args[0])
		os.Exit(1)
	}

	reposDir := os.Args[1]
	port := uint16(8080)

	if len(os.Args) >= 3 {
		p, err := strconv.ParseUint(os.Args[2], 10, 16)
		if err != nil {
			log.Fatalf("invalid port: %v", err)
		}
		port = uint16(p)
	}

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	srv := githttp.NewServer(githttp.Config{
		ReposDir: reposDir,
		Address:  addr,
	})

	log.Fatal(srv.Serve())
}
