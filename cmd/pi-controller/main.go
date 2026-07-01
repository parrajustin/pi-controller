package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"
)

const (
	publicKeyFile = "publickey.pem"
)

func calculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func checkDocker() {
	log.Println("Checking if Docker is installed...")
	if _, err := exec.LookPath("docker"); err != nil {
		log.Println("Docker is not installed. Require it be installed...")
		os.Exit(1)
	}

	log.Println("Checking Docker service status...")
	cmd := exec.Command("systemctl", "is-active", "--quiet", "docker")
	if err := cmd.Run(); err != nil {
		log.Println("Docker service is not running. Attempting to start it via systemd...")
		startCmd := exec.Command("sudo", "systemctl", "start", "docker")
		startCmd.Stdout = os.Stdout
		startCmd.Stderr = os.Stderr
		if err := startCmd.Run(); err != nil {
			log.Fatalf("Error: Failed to start Docker service: %v", err)
		}

		verifyCmd := exec.Command("systemctl", "is-active", "--quiet", "docker")
		if err := verifyCmd.Run(); err != nil {
			log.Fatalf("Error: Docker service failed to run after start attempt.")
		}
	}
	log.Println("Docker service is running.")
}

func checkAndReplaceSplash() {
	srcMD5, err := calculateMD5("splash.png")
	if err != nil {
		log.Printf("Failed to calculate MD5 of splash.png: %v", err)
		return
	}

	destMD5, err := calculateMD5("/usr/share/plymouth/themes/pix/splash.png")
	if err != nil {
		log.Printf("Failed to calculate MD5 of dest splash.png: %v", err)
	}

	if srcMD5 != destMD5 {
		log.Println("Splash screen MD5 mismatch, replacing...")
		script := `mv /usr/share/plymouth/themes/pix/splash.png /usr/share/plymouth/themes/pix/splash_default.png
cp splash.png /usr/share/plymouth/themes/pix`

		cmd := exec.Command("bash", "-c", script)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Printf("Failed to replace splash screen: %v", err)
		} else {
			log.Println("Successfully replaced splash screen.")
		}
	} else {
		log.Println("Splash screen MD5 matches, no replacement needed.")
	}
}

func main() {
	log.Println("Starting pi-controller...")

	if _, err := os.Stat(publicKeyFile); os.IsNotExist(err) {
		log.Fatalf("Fatal: %s is missing from the directory", publicKeyFile)
	}

	checkDocker()

	checkAndReplaceSplash()

	// Main application loop
	// For now, it just simulates the pi-controller running
	for {
		log.Println("pi-controller is running in foreground...")
		time.Sleep(10 * time.Second)
		log.Println("tick!")
	}
}
