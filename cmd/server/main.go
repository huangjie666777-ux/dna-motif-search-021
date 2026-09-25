package main

import (
	"log"
	"net/http"
	"os"

	"github.com/huangjie666777-ux/dna-motif-search-021/internal/api"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	r := api.NewRouter()
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
