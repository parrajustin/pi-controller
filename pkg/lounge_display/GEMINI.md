# Lounge Display - Architecture & Developer Guide

This directory (`pkg/lounge_display`) contains the entire software stack for the Lounge Display device, integrating both frontend web applications and backend Go servers into a single orchestratable unit.

## System Architecture & Process Flow

The system is designed to run inside a single Docker container to simplify deployment, but internally it orchestrates multiple applications to achieve its goal:

1. **Initial Setup (OAuth Flow)**:
   - When the container starts (via `start.sh`), it first launches the **`setup_server`**.
   - The `setup_server` listens on port `8080` and serves the **`setup_web`** frontend.
   - This setup phase ensures the device has internet access and guides the user through authenticating with Google Calendar to generate `credentials.json` and `token.json`.
   - Once the OAuth token is successfully fetched and verified, `setup_server` safely exits.

2. **Main Display Application**:
   - After `setup_server` finishes, `start.sh` immediately hands over execution to the **`display_server`**.
   - The `display_server` serves the primary frontend (**`display_control_web`** and **`meeting_control_web`**) on port `8080` and acts as the backend API, utilizing the generated tokens to interact with Google APIs.

### Sub-Components

For detailed documentation on the individual components, please refer to their respective `GEMINI.md` guides:

* [meeting_control_web/GEMINI.MD](meeting_control_web/GEMINI.MD) - The frontend interface for controlling meetings.
* [setup_web/GEMINI.md](setup_web/GEMINI.md) - The frontend interface for the initial device setup and OAuth flow.
* [setup_server/GEMINI.md](setup_server/GEMINI.md) - The Go backend responsible for obtaining Google OAuth tokens.

*(Note: `display_server` and `display_control_web` are the other core components but currently lack dedicated GEMINI guides).*

## How to Run

To build and run the complete system locally, use the provided `run.sh` script:

```bash
cd pkg/lounge_display
./run.sh
```

This script automates the following process:
1. **Host IP Detection**: It determines your machine's true Local IP Address (e.g., `192.168.1.100`) by querying your active routing table.
2. **Builds the Docker Image**: Builds the multi-stage `Dockerfile`, compiling all frontends and Go binaries into a single production image.
3. **Starts the Container**: Runs the resulting image, mapping port `8080` and passing in the detected `HOST_IP`.

### The Importance of the `HOST_IP` Environment Variable

When running this container (especially on systems using Docker Desktop), the container is often placed on an isolated virtual machine network (e.g., `192.168.65.x`). 

However, during the Google OAuth flow, the `setup_server` needs to generate an authorization URL with a specific `redirect_uri` that points back to the device. If the server uses the Docker VM IP, the OAuth flow will fail because your browser cannot reach `192.168.65.x`.

To resolve this, the Docker container **requires** the `HOST_IP` environment variable to be set to your actual network IP (e.g., `192.168.1.100`). The `setup_server` explicitly reads this variable to construct a functional `redirect_uri` that your browser and Google can use to return the authorization token. The `run.sh` script handles this injection automatically.

## Node Engine Architecture

Both `setup_server` and `display_server` use a highly extensible state machine (Node Engine) for browser automation and application progression. Instead of relying on a single large procedure, logic is broken up into discrete `Node` components.

### Core Node Lifecycle
1. **Setup**: Runs exactly once when entering the node (useful for mounting HTTP endpoints).
2. **Work**: The core execution logic for that state (e.g., extracting an auth code, interacting with the DOM).
3. **DoneCheck**: Evaluates if this specific stage's goal is complete. If it fails, the engine safely restarts back to the `DefaultNode`.
4. **PreCheck (Next Nodes)**: Evaluates possible paths forward from a node's `Next` array. The engine automatically navigates to the first node whose `PreCheck` returns `true`.
5. **Teardown**: Cleanup operations before transitioning to the next node.

### Rest Nodes, Timeouts, and Non-Blocking Rules
Because some stages require indefinite waiting (e.g., human user input, or sitting in a Google Meet), nodes can be explicitly marked as `IsRestNode: true`.
- **Rest Nodes**: Instead of causing an infinite evaluation loop that spams terminal logs, Rest Nodes instruct the engine to cleanly pause execution while waiting for a valid forward transition (`Next`). While resting, the engine continuously verifies that the current node's `PreCheck` remains valid; if it fails, the engine seamlessly resets. Rest Nodes ignore timeouts.
- **Normal Nodes**: Any non-rest node is strictly bounded by `s.NodeTimeout` (configured to 10-20 minutes). If a non-rest node fails to transition to a new node before the timeout expires, the engine aborts the hanging process and resets to `DefaultNode`.

> [!WARNING]
> **Strict Non-Blocking Rule**: Regardless of whether a node is a Rest Node or a Normal Node, its `Work`, `DoneCheck`, and `PreCheck` methods MUST NOT contain infinite polling loops, blocking channel reads (`<-chan`), or blocking network calls. 
> 
> The Node Engine relies on continuously iterating its main execution loop to evaluate Global Transitions (like the "Touchpad Control" or "Reboot" features). If a node's `Work` method blocks, the entire state machine freezes, and the system becomes completely unresponsive to WebSocket commands and global state changes. 
> 
> For long-running waits (e.g. waiting for a user to complete 2FA), set `IsRestNode: true`, let `Work` return immediately, and rely on the engine's built-in tick loop to evaluate the subsequent node's `PreCheck`.

### Automated Dumps & Artifacts
When transitioning between nodes, the engine automatically leverages Chrome CDP to capture `.png` screenshots and `.html` DOM snapshots (`pre` and `post` Work execution). These artifacts are saved into the `logs/` directory sequentially (e.g., `0008_display_join_meeting_page_pre_screenshot.png`), providing a perfect visual timeline of exactly what the headless browser experienced.

## Chrome CDP & Google Meet Automation

Google Meet employs advanced anti-bot heuristics in its UI. Specifically, its interactive elements (like the "Join anyway" or "Ask to join" buttons) actively inspect the `isTrusted` property on JavaScript events.
* **Avoid `.click()`**: Using standard JavaScript evaluation in `chromedp.Evaluate` (e.g. `document.querySelector('button').click()`) will fail silently in Meet because the simulated event is untrusted.
* **CDP Clicks**: You MUST use low-level CDP commands like `chromedp.Click` (`Input.dispatchMouseEvent`). This directly commands the Chrome binary to perform a hardware-level click on the element, effortlessly bypassing `isTrusted` validation. 
* **State Polling**: Meet UI components are heavily dynamic. When navigating to a meeting, the engine uses robust polling loops (using JS evaluation injected into the page) to patiently wait for target buttons to drop their `disabled` or `aria-disabled="true"` flags before initiating the trusted CDP click.

## Websocket APIs

The `display_server` communicates with the UI clients primarily through WebSockets. The complete documentation of all emitted events and client-invoked methods (including when they become available and when they are removed during state transitions) is maintained in `pkg/lounge_display/display_server/ws_documentation.md`.

> [!IMPORTANT]
> **WS Documentation Requirement**: Any time a new WebSocket event is added, or an existing WebSocket method is removed/modified in the engine, you MUST update the [ws_documentation.md](display_server/ws_documentation.md) file to reflect these changes.

## Testing Requirements

> [!CAUTION]
> **All tests are required to pass** after making any modifications in the `pkg/lounge_display` directory. Ensure you run the complete test suite before finalizing your work.

### Web Applications

The web frontend applications (`display_control_web`, `meeting_control_web`, `setup_web`) use standard NPM tooling for testing, which encompasses unit tests, Playwright visual tests, ESLint linting, and TypeScript compilation checks (tsc). 

To run the complete web test suite for a given frontend component, navigate to its directory and run:

```bash
cd display_control_web
npm run test
```
*(Repeat for `meeting_control_web` and `setup_web`)*

### Go Server & Integration Tests

The Go components (`display_server`, `setup_server`, `calendarclient`, etc.) include unit tests and end-to-end integration tests.

To run all Go tests and integration tests across the entire `pkg/lounge_display` directory, navigate to the `pkg/lounge_display` directory and run:

```bash
cd pkg/lounge_display
go test ./...
```

This will automatically execute standard Go unit tests as well as the test suite located in `pkg/lounge_display/integration_test`.
