# Pi Controller

A controller service designed to run on a Raspberry Pi (or x86_64 system) to manage and control its display and environment.

## One-Line Installation

The easiest way to install and start the `pi-controller` service is to use the automated install script.

First, navigate to the directory where you want the binaries to live (e.g., `/opt/pi-controller`), and run the following command:

```bash
curl -sL https://raw.githubusercontent.com/parrajustin/pi-controller/main/install.sh | sudo bash
```

### What this script does:
1. Checks for root privileges.
2. Detects your system's architecture (`x86_64` or `aarch64`).
3. Downloads the latest release tarball directly from GitHub.
4. Extracts the binaries into your current working directory.
5. Generates a `pi-controller.service` systemd file targeting this directory.
6. Reloads systemd, enables the service on boot, and starts it in the background.

---

## Manual Installation

If you prefer to configure everything yourself:

1. Download the latest `.tar.gz` for your architecture from the [Releases](https://github.com/parrajustin/pi-controller/releases) page.
2. Extract the archive to your desired location (e.g., `/opt/pi-controller`).
3. Download the standalone `pi-controller.service` file from this repository.
4. Edit `pi-controller.service` and update the `WorkingDirectory` and `ExecStart` paths to match your installation location.
5. Move the service file: `sudo cp pi-controller.service /etc/systemd/system/`
6. Reload the daemon: `sudo systemctl daemon-reload`
7. Enable and start: `sudo systemctl enable --now pi-controller.service`

## Checking Logs

Once installed, you can view the live output of the controller by running:
```bash
journalctl -u pi-controller.service -f
```
