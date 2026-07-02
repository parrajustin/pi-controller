# Lounge Display Web Application

This directory (`pkg/lounge_display/meeting_control_web`) contains the frontend web application designed for a "Lounge Display" (e.g., a meeting room or lodge display screen). 

## Architecture & Tech Stack
- **Framework**: Built with **Lit** (Web Components), enabling a modular and highly performant architecture.
- **Styling**: Utilizes **Material Design 3** aesthetics, including animations, ripples from `@material/web`, and custom CSS tokens.
- **Time Logic**: Relies on **`date-fns`** for robust parsing and interval calculations to dynamically determine which meetings are currently active.
- **Build System**: Uses **`esbuild`** for lightning-fast bundling of TypeScript and assets (like `.png` splash images) into a single `dist/bundle.js`. Development uses `live-server` running concurrently with `esbuild --watch`.

## Component Structure
The source code is primarily located inside the `src/` directory.

### `src/app.ts` (`<lounge-display>`)
The root application component. It acts as the central state manager:
- Sets up the main layout: Header (logo & dynamic clock), Middle (Meeting List), and Bottom (Control Bar).
- Maintains a real-time 1-second interval clock to track the exact current time.
- Manages the `meetings` state array (mocked data).
- Automatically updates which meeting is `isActive` and filters the view to only show upcoming meetings (plus a maximum of 1 immediate past meeting).

### `src/components/meeting-list.ts` & `meeting-entry.ts`
Responsible for displaying the day's schedule. 
- `<meeting-list>` maps over the active meetings.
- `<meeting-entry>` displays individual rows (Start time, Name, Status). It applies special active styling (raised elevation, background color changes) if the meeting is currently happening, and features a `<md-ripple>` effect for touch interactions.

### `src/components/bottom-bar.ts`
The control center at the bottom of the screen.
- Features an input area to "Enter a code or nickname" and action icon buttons.
- Implements an immersive **Material 3 Fullscreen Overlay**. When the user taps to type, it takes over the screen, showing a massive input field at the top and animating an on-screen keyboard sliding up from the bottom.
- Manages syncing text between the overlay and the bottom bar.

### `src/components/virtual-keyboard.ts`
A custom-built, touch-friendly on-screen keyboard.
- Features standard QWERTY layout, backspace, left/right caret navigation, and a prominent blue Enter key.
- Includes a dedicated "Dismiss" button to safely close the fullscreen overlay.
- Dispatches custom `@key-pressed` events to the parent `bottom-bar` component to mutate input values.

## Package Management Instructions

For any future package installations or modifications in this project, **use `pnpm`** instead of `npm`.

### Commands
- Install all dependencies: `pnpm install`
- Add a dependency: `pnpm add <package>`
- Add a dev dependency: `pnpm add -D <package>`

`npm` and `yarn` should not be used.
