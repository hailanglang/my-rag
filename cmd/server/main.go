package main

import (
	"log"
	"net/http"
	"os"

	"my-rag/internal/httpapi"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		addr = v
	}
	log.Printf("listening %s", addr)
	if err := http.ListenAndServe(addr, httpapi.NewRouter()); err != nil {
		log.Fatal(err)
	}
}
