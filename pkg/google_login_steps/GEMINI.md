# Google Login Steps via Chrome Remote Protocol

This project is a standalone Go utility designed to automate the process of authenticating a Google account (`lounge.room@mountainviewmasoniclodge.com`) into Google Meet. It operates by attaching to a running Chrome or Chromium instance via the Chrome DevTools Protocol (CDP).

## Overview

The binary connects to Chrome using the remote debugging port (typically `ws://127.0.0.1:9222`). By leveraging the `github.com/chromedp/chromedp` library, the program can read the DOM, inject keystrokes, and navigate through Google's dynamic authentication flows.

Because Google's login flows are highly unpredictable—often A/B testing different UI variants or branching based on existing session state (e.g., rendering a "Choose an Account" page vs. an empty email input)—a simple linear script is insufficient. To gracefully handle this, the project utilizes a **Node-Based Graph Architecture**.

## Node-Based Graph Architecture

The core of the logic is built around the `RunEngine` function and the `Node` struct. Rather than strictly assuming the next page is always `Y` after interacting with page `X`, the engine evaluates a tree of possible outcomes.

### The `Node` Struct

```go
type Node struct {
	Name      string
	PreCheck  func(s *StateContext) bool
	Work      func(s *StateContext) error
	DoneCheck func(s *StateContext) error
	Next      []*Node
}
```

- **`PreCheck`**: A fast, non-blocking evaluation (e.g., querying the DOM or strictly checking `url.Host`) to determine if the browser is currently in a state that matches this node.
- **`Work`**: The actions to take if this node is chosen as the active node (e.g., typing a password, clicking a "Next" button).
- **`DoneCheck`**: An optional long-running polling loop that ensures the `Work` resulted in a successful state transition before allowing the engine to proceed. If it times out (e.g., waiting 10 minutes for a user to complete 2FA), the engine fails gracefully.
- **`Next`**: An array of potential subsequent nodes. The engine will rapidly poll the `PreCheck` of all nodes in this array until one evaluates to `true`.

### How the Engine Runs

1. The engine executes the active node's `Work` and `DoneCheck`.
2. It captures debug artifacts (HTML dump and screenshot) of the resulting state.
3. It iterates over the `Next` array, polling each node's `PreCheck` for up to 10 seconds.
4. As soon as a `PreCheck` returns true, that node becomes the active node, and the cycle repeats.
5. If no `PreCheck` matches within the timeout, the engine declares it is lost, captures a `_failed_dump` artifact, and halts for manual intervention via `stdin`.

## Artifacts and Debugging

To facilitate debugging, the engine aggressively dumps state at every step.
By default, artifacts are saved into the `logs/` directory, but this can be overridden via the `-logs-dir` CLI flag. The directory is automatically wiped clean on startup to prevent stale dumps from muddying the timeline.

Artifacts follow the naming convention:
`{0-PADDED-STEP}_{NODE_NAME}_{dump.html|screenshot.png}`
*(e.g., `0004_email_input_page_screenshot.png`)*

This chronological ordering allows developers to track exactly what the engine saw at every decision point and diagnose why a specific path was chosen.

## CLI Flags

- `-logs-dir string`: Specifies the directory to store HTML dumps and screenshots (default `"logs"`).
- `-graph`: Prints the node state machine graph in Mermaid format and exits immediately. This can be pasted into [Mermaid Live Editor](https://mermaid.live/) to visualize the active flow architecture.
