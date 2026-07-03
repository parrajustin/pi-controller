# Setup Server Documentation

This document outlines the architecture, flow, and APIs of the `setup_server` module in the Lounge Display project.

## Overview
The `setup_server` is a lightweight Go application designed to orchestrate the initial bootstrapping phase of the Lounge Display device. Its primary responsibility is to ensure the device has an active internet connection, gather Google Cloud credentials (`credentials.json`), execute the Google OAuth2 flow to generate a `token.json`, and finally verify Calendar API access before handing control over to the main `display_server`.

## Architecture & Libraries
- **Language**: Go (statically compiled for Alpine Linux).
- **HTTP Server**: Uses the standard `net/http` package with custom CORS/CSP header middleware.
- **OAuth Handling**: Uses `golang.org/x/oauth2` and `golang.org/x/oauth2/google`.
- **Google API**: Uses `google.golang.org/api/calendar/v3` to test the validity of the generated tokens.
- **Concurrency**: Utilizes Go channels (`credChan`, `authCodeChan`) to block the main thread execution while the HTTP server listens for incoming configuration payloads in the background.

## Application Flow & States
The server initializes an HTTP router in a background goroutine and then progresses through a linear sequence of setup phases on the main thread:

1. **Credentials Phase**: 
   - Checks if `credentials.json` exists in `OAUTH_DIR` (defaults to current directory).
   - If missing, it blocks and waits for a POST request on `/api/cred`.
   - Once received, it writes the file securely to disk.

2. **Token Phase**:
   - Parses the credentials file to configure the OAuth client.
   - Sets the redirect URL dynamically to `http://<LOCAL_IP>:<PORT>/api/token` to capture the callback.
   - Checks if `token.json` exists.
   - If missing, generates an Auth URL and blocks, waiting for a code on `/api/token`.
   - Exchanges the code for an offline token and saves it to `token.json`.

3. **Verification Phase**:
   - Initializes the Calendar API service using the generated token.
   - Fetches the next 10 events from the primary calendar to verify API access.
   - Sets `setupReady = true`, triggering the web frontend to redirect.
   - Sleeps for 3 seconds and exits, allowing the system/bash script to launch the main `display_server`.

## Endpoints
| Endpoint | Method | Description |
|---|---|---|
| `/api/ip` | GET | Returns the local IP address of the Pi/Device. |
| `/api/status` | GET | Returns `{"status": "pending"}` or `{"status": "ready"}`. |
| `/api/has_wifi` | GET | Tests internet connection by pinging Google. |
| `/api/has_cred` | GET | Returns boolean indicating if `credentials.json` is present on disk. |
| `/api/cred` | POST | Accepts raw JSON payload containing credentials and pushes it to `credChan`. |
| `/api/auth_url` | GET | Returns the dynamically generated Google OAuth URL. |
| `/api/token` | GET/POST | Accepts the OAuth authorization code, either via a query param (Google Redirect) or JSON payload (Frontend POST). |
