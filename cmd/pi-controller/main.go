package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/parrajustin/pi-controller/pkg/logger"
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
	slog.Info("Checking if Docker is installed...")
	if _, err := exec.LookPath("docker"); err != nil {
		slog.Info("Docker is not installed. Require it be installed...")
		os.Exit(1)
	}

	slog.Info("Checking Docker service status...")
	cmd := exec.Command("systemctl", "is-active", "--quiet", "docker")
	if err := cmd.Run(); err != nil {
		slog.Info("Docker service is not running. Attempting to start it via systemd...")
		startCmd := exec.Command("sudo", "systemctl", "start", "docker")
		startCmd.Stdout = os.Stdout
		startCmd.Stderr = os.Stderr
		if err := startCmd.Run(); err != nil {
			logger.Fatalf("Error: Failed to start Docker service: %v", err)
		}

		verifyCmd := exec.Command("systemctl", "is-active", "--quiet", "docker")
		if err := verifyCmd.Run(); err != nil {
			logger.Fatalf("Error: Docker service failed to run after start attempt.")
		}
	}
	slog.Info("Docker service is running.")
}

func checkAndReplaceSplash() {
	srcMD5, err := calculateMD5("splash.png")
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to calculate MD5 of splash.png: %v", err))
		return
	}

	destMD5, err := calculateMD5("/usr/share/plymouth/themes/pix/splash.png")
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to calculate MD5 of dest splash.png: %v", err))
	}

	if srcMD5 != destMD5 {
		slog.Info("Splash screen MD5 mismatch, replacing...")
		script := `mv /usr/share/plymouth/themes/pix/splash.png /usr/share/plymouth/themes/pix/splash_default.png
cp splash.png /usr/share/plymouth/themes/pix
plymouth-set-default-theme --rebuild-initrd pix`

		cmd := exec.Command("bash", "-c", script)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			slog.Error(fmt.Sprintf("Failed to replace splash screen: %v", err))
		} else {
			slog.Info("Successfully replaced splash screen.")
		}
	} else {
		slog.Info("Splash screen MD5 matches, no replacement needed.")
	}
}

func stopContainers() {
	slog.Info("Stopping all Docker containers...")
	cmd := exec.Command("bash", "-c", "docker stop $(docker ps -q)")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.Info(fmt.Sprintf("Command returned an error (likely no containers to stop): %v", err))
	} else {
		slog.Info("Successfully stopped Docker containers.")
	}
}

func startDockerCompose() {
	slog.Info("Starting docker compose services...")
	cmd := exec.Command("docker", "compose", "-f", "docker/docker-compose.yml", "up", "--build", "--force-recreate", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.Error(fmt.Sprintf("Failed to run docker compose: %v", err))
	} else {
		slog.Info("Successfully started docker compose services.")
	}
}
func handleReboot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	slog.Info("Reboot API called, issuing reboot command...")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Rebooting system..."))

	// Delay the reboot slightly to allow the HTTP response to be sent
	go func() {
		time.Sleep(1 * time.Second)
		cmd := exec.Command("sudo", "reboot")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			slog.Error(fmt.Sprintf("Failed to reboot: %v", err))
		}
	}()
}

func main() {
	logger.Init("pi-controller")
	slog.Info("Starting pi-controller...")

	shutdown, err := InitTelemetry(context.Background())
	if err != nil {
		slog.Error("Failed to initialize telemetry", "error", err)
	} else {
		defer func() {
			if err := shutdown(context.Background()); err != nil {
				slog.Error("Failed to shutdown telemetry", "error", err)
			}
		}()
	}

	if _, err := os.Stat(publicKeyFile); os.IsNotExist(err) {
		logger.Fatalf("Fatal: %s is missing from the directory", publicKeyFile)
	}

	checkDocker()

	checkAndReplaceSplash()

	stopContainers()

	startDockerCompose()

	http.HandleFunc("/reboot", handleReboot)
	go func() {
		slog.Info("Starting API server on :6060")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			slog.Error(fmt.Sprintf("API server failed: %v", err))
		}
	}()

	// Main application loop
	// For now, it just simulates the pi-controller running
	for {
		slog.Info("pi-controller is running in foreground...")
		time.Sleep(10 * time.Second)
		slog.Info("tick!")
	}
}
