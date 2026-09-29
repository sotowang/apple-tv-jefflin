package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	c := http.Client{Timeout: 3 * time.Second}
	r, e := c.Get("http://127.0.0.1:8080/health")
	if e != nil {
		os.Exit(1)
	}
	r.Body.Close()
	if r.StatusCode != 200 {
		os.Exit(1)
	}
}
