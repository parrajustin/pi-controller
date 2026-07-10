# Websocket API Documentation

The Display Server communicates with the frontend client UI through a single WebSocket endpoint located at `/ws`. Communication is bidirectional, featuring both Server-Emitted Events (broadcasts) and Client-Invoked Methods (RPCs).

## Server-Emitted Events

These events are pushed continuously by the server to all connected WebSocket clients without a prompt.

### 1. `state_update`
- **When it is emitted:** Whenever the state engine updates its status. This happens constantly during transitions between Nodes, or when changing phases within a node (e.g., `pre-setup`, `setup`, `pre-work`, `work`, `done-check`, `transitioning`). It is also emitted when properties like the meeting code change or the system declares itself "setup ready".
- **Payload Format:**
  ```json
  {
    "type": "state_update",
    "payload": {
      "current_node": "Name of the Active Node (e.g., Meet Landing Page)",
      "meeting_code": "xyz-abcd-efg",
      "setup_ready": true,
      "phase": "work",
      "setup_phase": 5
    }
  }
  ```

### 2. `response`
- **When it is emitted:** Whenever the UI invokes a client method (see below) and provides a unique `id` field in the request. The server will process the method and reply with a `response` type matching that ID.
- **Payload Format (Success):**
  ```json
  {
    "type": "response",
    "id": "<request-id>",
    "payload": { ...method specific output... }
  }
  ```
- **Payload Format (Error):**
  ```json
  {
    "type": "response",
    "id": "<request-id>",
    "error": "Error message details"
  }
  ```

---

## Client-Invoked Methods (Handlers)

Clients can invoke methods by sending JSON to the `/ws` endpoint in the following format:
```json
{
  "type": "<method_name>",
  "id": "12345", 
  "payload": { ... }
}
```
**CRITICAL NOTE:** The server dynamically adds and removes these methods based on which Node is currently active. If the UI attempts to invoke a method that is not currently active, the server will reply with an `unknown message type` error.

### 1. `has_wifi`
- **When it is available:** Added to the system during the **entire Google Setup Phase** (from `Init Server` through to `Two-Factor Authentication`).
- **When it is removed:** Automatically removed when the Setup Phase completes and the display transitions to the Google Meet interface (e.g. starting at `Init CDP`).
- **Request Payload:** None `null`
- **Response Payload:** `{"internetAccess": true/false}`

### 2. `get_auth_url`
- **When it is available:** Added when the server enters the `AuthTokenNode`.
- **When it is removed:** Removed as soon as the `AuthTokenNode` completes (when the server receives the token).
- **Request Payload:** None `null`
- **Response Payload:** `{"url": "https://accounts.google.com/o/oauth2/auth?..."}`

### 3. `submit_token`
- **When it is available:** Added alongside `get_auth_url` during the `AuthTokenNode`.
- **When it is removed:** Removed when the `AuthTokenNode` completes.
- **Request Payload:** `{"code": "auth-code-string"}`
- **Response Payload:** `{"status": "ok"}`

### 4. `submit_password`
- **When it is available:** Added when the server enters the `PasswordInputNode` (when Google prompts for the account password).
- **When it is removed:** Removed immediately when transitioning out of the password screen (either successfully logging in, or failing due to a wrong password).
- **Request Payload:** `{"password": "user-inputted-password"}`
- **Response Payload:** `{"status": "ok"}`

### 5. `join_meeting`
- **When it is available:** Added when the server is idle on the `Meet Landing Page` Node.
- **When it is removed:** Removed as soon as the user invokes it and the system transitions toward joining the meeting.
- **Request Payload:** `{"code": "xyz-abcd-efg"}`
- **Response Payload:** `{"status": "ok"}`

### 6. `button_state`
- **When it is available:** Added only when the user is actively inside a meeting room via the `InMeetingNode`.
- **When it is removed:** Removed as soon as the meeting ends or the user hangs up.
- **Request Payload:** None `null`
- **Response Payload:** 
  ```json
  {
    "in_meeting": true,
    "microphone": false,
    "camera": true,
    "hand": false
  }
  ```

### 7. `click_button`
- **When it is available:** Added alongside `button_state` during the `InMeetingNode`.
- **When it is removed:** Removed as soon as the meeting ends or the user hangs up.
- **Request Payload:** `{"button": "microphone"}` (valid options: `microphone`, `camera`, `hand`, `hangup`)
- **Response Payload:** `{"status": "ok"}`
