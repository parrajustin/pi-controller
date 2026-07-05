# Lounge Display Ecosystem Architecture

This document describes the orchestration of the three main components of the Lounge Display system, how they interact, and how to execute the entire ecosystem using the integrated Docker Compose setup.

## The 3 Main Components

1. **`lounge_display` (Setup Server & Orchestrator)**
   - **Role**: The core brain of the system. It runs a Go-based state machine that serves web frontends (like `setup_web`), handles Google OAuth logic, and orchestrates interactions with Google Meet. 
   - **Mechanism**: It operates the Chrome DevTools Protocol (CDP) to programmatically log in to Google Meet and manage meeting states. It also dynamically generates QR codes for users on the local network to pair with the display.

2. **`kiosk_debug` (Headless Chromium)**
   - **Role**: A standalone, headless Chromium browser container.
   - **Mechanism**: It runs a stable Chromium environment tailored for media handling (`--use-fake-ui-for-media-stream`, etc.). It exposes its remote debugging port (CDP) so that `lounge_display` can connect to it, navigate, and interact with the DOM as if it were a real user.

3. **`token-receiver`**
   - **Role**: A specialized service dedicated to securely receiving tokens.
   - **Mechanism**: Exposes a REST API on port `7070` to handle token ingestion independently from the main lounge display state machine.

## How They Work Together

The system leverages Docker Compose to network these components in an isolated, secure way:
- **Web UI & User Interaction**: Users access the `lounge_display` frontend on port `8080`. The Go server uses the host's actual LAN IP (`HOST_IP`) to display QR codes, ensuring devices like mobile phones can reach the server over Wi-Fi.
- **Browser Automation**: When the state machine reaches the Google Meet phase, `lounge_display` performs a DNS lookup on the `KIOSK_IP` (which maps to the `kiosk_debug` container name in the compose network). It then establishes a WebSocket connection over `CDP_PORT` to remotely drive the browser.
- **Token Handling**: The `token-receiver` runs passively on port `7070`, validating and handling external tokens when required by the broader ecosystem.

## Environment Variables

### `lounge_display`
* `HOST_IP`: The actual LAN IP address of the host machine (e.g., `192.168.1.107`). **Crucial** for the `/api/ip` endpoint, which generates the correct QR code URL for external devices to scan.
* `KIOSK_IP`: The hostname of the browser container (`kiosk_debug` by default). The Go server uses `net.LookupIP` on this variable to resolve the internal docker IP and bypass Chrome's strict Host Header security checks.
* `CDP_PORT`: The internal Chrome DevTools Protocol port to connect to (default `9223`).
* `TOKEN_ENCRYPTION_KEY`: A secret key used to encrypt/decrypt sensitive data.
* `SKIP_SETUP`: Boolean flag to bypass the interactive setup phases.

### `token-receiver`
* `SERVER_ADDR`: The bind address and port for the receiver (e.g., `0.0.0.0:7070`).

## Assumptions & Requirements

* **Network**: The host machine must have a valid default route to the internet (used by `ip -4 route get 8.8.8.8` to deduce the `HOST_IP`).
* **Ports**: Ports `8080` (Lounge Display), `9222` (Kiosk Debug external mapping), and `7070` (Token Receiver) must be free on the host.
* **Resources**: The `kiosk_debug` container is provided a `2g` shared memory size (`--shm-size=2g`) to prevent Chromium from crashing (OOM) during complex page renders.

## Process for Execution

To launch the entire ecosystem seamlessly:

1. Navigate to the `pkg` directory:
   ```bash
   cd pkg
   ```
2. Execute the test launcher script:
   ```bash
   ./run_test_compose.sh
   ```

**What the script does:**
1. It automatically calculates the `HOST_IP` by determining the active LAN interface.
2. It sets up the required output directories (`kiosk_debug/chrome-data`, `lounge_display/oauth_test`, `lounge_display/logs`).
3. It exports the environment variables and triggers `docker compose -f docker-compose-test.yaml up --build`, spinning up all three components simultaneously in the foreground.
