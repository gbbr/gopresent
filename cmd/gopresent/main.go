package main

import (
	"log"
	"net/http"

	"github.com/gbbr/gopresent"
)

func main() {
	opts := gopresent.Options{}
	log.Fatal(http.ListenAndServe(":8080", gopresent.NewApp(opts)))
}
