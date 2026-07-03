# Setup Web Documentation

This document outlines the architecture, logic, and design of the `setup_web` frontend interface in the Lounge Display project.

## Overview
The `setup_web` module is a lightweight Single Page Application (SPA) built with Web Components. It serves as the user-facing interface during the device's bootstrapping phase, guiding the user through connecting to WiFi, uploading credentials, and linking the Google Auth token.

## Architecture & Libraries
- **Framework**: `lit` (LitElement) and `lit/decorators.js` for building reactive web components.
- **UI Components**: `@material/web` (Material 3) for checkboxes, progress bars, and basic theming.
- **Utilities**: 
  - `standard-ts-lib`: Sourced directly via Git (`git+https://github.com/parrajustin/standard-ts-lib.git`). We specifically leverage `WrapPromise` for robust error handling without relying on nested `try/catch` blocks. (See [standard-ts-lib/GEMINI.md](file:///home/jrparra/git/standard-ts-lib/GEMINI.md))
  - `qrcode`: Used to dynamically generate frontend QR codes pointing to local network URLs.
- **Build System**: Managed by `pnpm` and `esbuild`, configured to compile multiple entry points (`app.ts` and `upload.ts`) into the `dist/` directory.

## Pages & Entry Points

### 1. Main Setup Page (`index.html` -> `src/app.ts`)
The main dashboard that polls the `setup_server` and displays a sequential checklist.
- **State Machine**: 
  - Runs a centralized `startPolling()` loop every 15 seconds.
  - Evaluates independent `checkStageX()` methods leveraging early-exit returns for cleaner logic.
- **Stages**:
  - **Stage 1 (WiFi)**: Hits `/api/has_wifi`. Fails if unreachable.
  - **Stage 2 (Credentials)**: Hits `/api/has_cred`. If missing, dynamically generates a QR code to the `upload.html` page using the device's local network IP.
  - **Stage 3 (Auth Token)**: Hits `/api/status`. Waits for the server to confirm it has secured calendar access.
- **Animations**: Uses flexbox and CSS transitions. Completed stages receive a `.completed-hide` class, smoothly collapsing their `max-height` to 0 and fading out, keeping the UI focused and uncluttered.

### 2. Standalone Upload Page (`upload.html` -> `src/upload.ts`)
A dedicated, premium file upload interface that users interact with via their smartphones or laptops after scanning the QR code.
- **Features**: 
  - Glassmorphism design aesthetics (`backdrop-filter`, gradients, custom SVG icons).
  - Drag-and-drop file support with interactive hover/drag micro-animations.
  - Client-side validation to ensure the dropped file is a `.json` file and contains expected structural keys (e.g., `installed` or `web`).
- **Data Flow**: Reads the file contents and securely POSTs the raw JSON string to `/api/cred`.

## Design Aesthetics
The web interface adheres to strict premium design guidelines:
- **Colors**: Vibrant gradients (indigo to purple), dark mode backgrounds, and stark white contrast elements.
- **Typography**: Uses the 'Google Sans' and 'Inter' font families.
- **Layout**: Fluid flexbox layouts designed specifically to scale cleanly within constrained resolutions (e.g., 800x400 screens).
- **Interactions**: Uses micro-animations (e.g., hovering the drop-zone floats the icon) to make the UI feel alive and responsive.
