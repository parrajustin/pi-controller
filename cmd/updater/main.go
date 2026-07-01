package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"github.com/parrajustin/pi-controller/pkg/logger"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/mod/semver"
)

const (
	repoReleasesURL     = "https://api.github.com/repos/parrajustin/pi-controller/releases/latest"
	downloadURLTemplate = "https://github.com/parrajustin/pi-controller/releases/download/%s/%s"
	publicKeyFile       = "publickey.pem"
	versionFile         = "version.json"
	downloadDir         = "download"
	updateDir           = "update"
)

type VersionInfo struct {
	Version string `json:"version"`
}

type GitHubRelease struct {
	TagName string `json:"tag_name"`
}

func getArchiveName() string {
	arch := runtime.GOARCH
	if arch == "arm64" {
		return "pi-controller-aarch64.tar.gz"
	} else if arch == "amd64" {
		return "pi-controller-x86_64.tar.gz"
	}
	return fmt.Sprintf("pi-controller-%s.tar.gz", arch)
}

func main() {
	logger.Init("updater")
	if _, err := os.Stat(publicKeyFile); os.IsNotExist(err) {
		logger.Fatalf("Fatal: %s is missing from the directory", publicKeyFile)
	}

	slog.Info("Updater started. Checking for new releases...")

	currentVersion, err := getCurrentVersion()
	if err != nil {
		logger.Fatalf("Error reading current version: %v", err)
	}

	latestRelease, err := getLatestRelease()
	if err != nil {
		logger.Fatalf("Error fetching latest release: %v", err)
	}

	if semver.Compare(latestRelease.TagName, currentVersion) <= 0 {
		slog.Info(fmt.Sprintf("Already up-to-date (current: %s, latest: %s). Exiting.", currentVersion, latestRelease.TagName))
		return
	}

	slog.Info(fmt.Sprintf("New version found: %s (current: %s). Starting download...", latestRelease.TagName, currentVersion))

	if err := runUpdate(latestRelease.TagName); err != nil {
		logger.Fatalf("Update failed: %v", err)
	}

	slog.Info("Update downloaded and validated successfully!")
}

func getCurrentVersion() (string, error) {
	file, err := os.Open(versionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "v0.0.0", nil
		}
		return "", err
	}
	defer file.Close()

	var info VersionInfo
	if err := json.NewDecoder(file).Decode(&info); err != nil {
		return "", err
	}
	return info.Version, nil
}

func getLatestRelease() (*GitHubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, repoReleasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func runUpdate(version string) error {
	// Create download directory
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create download dir: %w", err)
	}
	// Clean download directory on exit
	defer os.RemoveAll(downloadDir)

	archiveName := getArchiveName()
	signatureName := archiveName + ".sig"

	archivePath := filepath.Join(downloadDir, archiveName)
	sigPath := filepath.Join(downloadDir, signatureName)

	slog.Info("Downloading archive...")
	if err := downloadFile(fmt.Sprintf(downloadURLTemplate, version, archiveName), archivePath); err != nil {
		return fmt.Errorf("failed to download archive: %w", err)
	}

	slog.Info("Downloading signature...")
	if err := downloadFile(fmt.Sprintf(downloadURLTemplate, version, signatureName), sigPath); err != nil {
		return fmt.Errorf("failed to download signature: %w", err)
	}

	slog.Info("Verifying signature...")
	if err := verifySignature(archivePath, sigPath, publicKeyFile); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	// Create update directory
	if err := os.MkdirAll(updateDir, 0755); err != nil {
		return fmt.Errorf("failed to create update dir: %w", err)
	}

	slog.Info("Extracting new files to update folder...")
	if err := extractTarGz(archivePath, updateDir); err != nil {
		return fmt.Errorf("failed to extract new files: %w", err)
	}

	return nil
}

func downloadFile(url, dest string) error {
	var err error
	for i := 0; i < 3; i++ {
		err = doDownloadFile(url, dest)
		if err == nil {
			return nil
		}
		slog.Error(fmt.Sprintf("Download failed (attempt %d/3): %v", i+1, err))
		time.Sleep(2 * time.Second)
	}
	return err
}

func doDownloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func verifySignature(dataPath, sigPath, pubKeyPath string) error {
	pubKeyData, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}

	block, _ := pem.Decode(pubKeyData)
	if block == nil {
		return fmt.Errorf("failed to parse PEM block containing the public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse DER encoded public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("not an RSA public key")
	}

	sig, err := os.ReadFile(sigPath)
	if err != nil {
		return fmt.Errorf("failed to read signature file: %w", err)
	}

	data, err := os.ReadFile(dataPath)
	if err != nil {
		return fmt.Errorf("failed to read data file: %w", err)
	}

	hashed := sha256.Sum256(data)
	return rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, hashed[:], sig)
}

func extractTarGz(gzipStream, dir string) error {
	f, err := os.Open(gzipStream)
	if err != nil {
		return err
	}
	defer f.Close()

	uncompressedStream, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer uncompressedStream.Close()

	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}
