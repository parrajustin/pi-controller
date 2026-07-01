package main

import (
	"fmt"
	"log/slog"
	"github.com/parrajustin/pi-controller/pkg/logger"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	updateDir    = "update"
	pollInterval = 10 * time.Minute
)

var filesToMove = []string{"pi-controller", "updater", "runner", "splash.png", "version.json", "publickey.pem"}

func main() {
	logger.Init("runner")
	slog.Info("Starting runner...")

	// 1. Synchronously run updater to fetch updates if any
	checkForUpdate()

	// 2. Check if update folder exists
	if _, err := os.Stat(updateDir); err == nil {
		slog.Info("Update directory found. Applying update...")
		applyUpdate()
		slog.Info("Update applied successfully. Shutting down to allow systemd to restart.")
		os.Exit(0)
	}

	// 3. Start update polling routine for subsequent checks
	go pollUpdates()

	// 4. Keep pi-controller running
	runPiControllerLoop()
}

func applyUpdate() {
	for _, f := range filesToMove {
		src := filepath.Join(updateDir, f)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		// Move file using Rename
		err := os.Rename(src, f)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to move %s: %v", f, err))
		}
		// ensure executable permissions for binaries
		if f == "pi-controller" || f == "updater" || f == "runner" {
			os.Chmod(f, 0755)
		}
	}
	os.RemoveAll(updateDir)
}

func runPiControllerLoop() {
	for {
		slog.Info("Starting pi-controller...")
		cmd := exec.Command("./pi-controller")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		err := cmd.Run()
		if err != nil {
			logger.Fatalf("pi-controller failed: %v. Failing runner as well.", err)
		}
		slog.Info(fmt.Sprintf("pi-controller exited without error. Restarting in 5 seconds..."))
		time.Sleep(5 * time.Second)
	}
}

func pollUpdates() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		if checkForUpdate() {
			slog.Info("Updater successfully downloaded a new release. Shutting down to apply update...")
			os.Exit(0)
		}
	}
}

// checkForUpdate runs the updater binary and returns true if an update folder was created.
func checkForUpdate() bool {
	slog.Info("Running updater to check for new releases...")
	
	cmd := exec.Command("./updater")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// We ignore the error here because we only care if the update folder was successfully created.
	// The updater binary itself will log any errors it encounters.
	_ = cmd.Run()
	
	if _, err := os.Stat(updateDir); err == nil {
		return true
	}
	return false
}
