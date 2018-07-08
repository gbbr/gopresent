package main

import (
	"log"
	"net/http"

	"github.com/gbbr/gopresent"
)

func main() {
	log.Fatal(http.ListenAndServe(":8080", gopresent.Handler()))
}
