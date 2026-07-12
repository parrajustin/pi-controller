# Lounge Display Ecosystem Architecture

This document describes the orchestration of the three main components of the Lounge Display system, how they interact, and how to execute the entire ecosystem using the integrated Docker Compose setup.

## The 3 Main Components

1. **`lounge_display` (Setup Server & Orchestrator)**
   - **Role**: The core brain of the system. It runs a Go-based state machine that serves web frontends (like `setup_web`), handles Google OAuth logic, and orchestrates interactions with Google Meet. 
   - **Mechanism**: It operates the Chrome DevTools Protocol (CDP) to programmatically log in to Google Meet and manage meeting states. It also dynamically generates QR codes for users on the local network to pair with the display.

2. **`kiosk_debug` (Headless Chromium)**
   - **Role**: A dual-display headless Chromium environment.
   - **Mechanism**: It runs a stable Chromium environment tailored for media handling. It orchestrates two independent Xvfb displays (`:0` and `:1`) with dedicated `matchbox-window-manager` and `x11vnc`/`websockify` instances for each. Both Chromium instances run heavily isolated with separate data directories and expose separate remote debugging ports (CDP) so `lounge_display` can drive them independently.

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

## Knowledge Base & Debugging Insights

Over the course of developing and debugging this ecosystem, several tricky architectural and functional issues were resolved. Here is a distilled knowledge base to assist future debugging:

### 1. Concurrency and WebSockets
- **Concurrent Writes Panic**: The Go backend utilizes `gorilla/websocket`. This library fundamentally does not support concurrent writes to the same connection. A race condition panicked the server when error responses and state updates were broadcast simultaneously.
- **Resolution**: A thread-safe helper `WriteWSJSON()` was implemented in `StateContext` using a global mutex (`s.mu.Lock()`). All WebSocket handlers must use this synchronized method.

### 2. State Machine Race Conditions
- **Frontend Phase Mismatches**: When new backend setup nodes (e.g., `WaitWebServerNode`) were added, the numerical indices of setup phases shifted upwards. The frontend (`setup-display.ts`) relied on hardcoded thresholds, causing the UI checklist to silently fail and disappear during the Google Login flow.
- **Resolution**: Refactored `SETUP_STEPS` in the frontend to explicitly map ranges using `startPhase` and `endPhase` properties, making the active-step checks robust against server-side node insertions.
- **CDP Connection Readiness**: The backend must not connect to Chromium CDP before the webserver and socat proxies are ready. A `WaitWebServerNode` ensures port 8080 is listening, and a `WaitForClientCallbackNode` blocks backend progression until the frontend connects to the WebSocket.

### 3. Chromium & Docker Quirks
- **CDP Context Cancellation Kills Chrome**: When managing the second display (`Init Display 2 CDP`), if the remote allocator context is canceled while no other active targets exist, Chrome will instantly exit, leaving VNC showing a black screen. Connections meant to keep a display alive must deliberately leak or store their contexts.
- **Lingering X11 Lock Files**: Container restarts can leave behind stale X11 lock files (e.g., `/tmp/.X0-lock`) and `SingletonLock` files in Chrome data directories, causing silent crashes. These must be aggressively cleaned at the top of the container entrypoint (`start.sh`).
- **GPU Contention**: Running two Chromium instances on the same host can trigger GPU context failures (`ContextResult::kTransientFailure`). Appending `--disable-gpu` to the Chromium launch arguments is required.

### 4. Advanced Diagnostics
- **Screen Recording**: A `--record_screens` flag in `run_test_compose.sh` automatically launches `ffmpeg -f x11grab` to record both Xvfb displays in 1-minute chunks, saving them to the `/recordings` volume. This is essential for diagnosing invisible headless UI bugs.
- **Log Spam Reduction**: For better visibility of application logs, the `otel-collector` (OpenTelemetry) container is configured with `logging: driver: none` in the docker-compose YAML.
