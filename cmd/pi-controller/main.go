package main

import (
	"log"
	"os"
	"time"
)

const (
	publicKeyFile = "publickey.pem"
)

func main() {
	log.Println("Starting pi-controller...")

	if _, err := os.Stat(publicKeyFile); os.IsNotExist(err) {
		log.Fatalf("Fatal: %s is missing from the directory", publicKeyFile)
	}

	// Main application loop
	// For now, it just simulates the pi-controller running
	for {
		log.Println("pi-controller is running in foreground...")
		time.Sleep(10 * time.Second)
		log.Println("tick!")
	}
}
