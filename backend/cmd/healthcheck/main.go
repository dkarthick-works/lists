// Command healthcheck is the container HEALTHCHECK probe.
package main

import (
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	resp, err := http.Get("http://localhost:" + port + "/api/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
