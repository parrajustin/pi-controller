#!/bin/bash

export LAUNCH_URL=${LAUNCH_URL:-"https://meet.google.com"}
export CHROME_DATA_DIR=${CHROME_DATA_DIR:-"/chrome-data"}

echo "Starting Xvfb on :0"
Xvfb :0 -screen 0 1536x960x24 -listen tcp -ac &
sleep 1

echo "Starting x11vnc"
x11vnc -display :0 -nopw -listen localhost -xkb -forever &
sleep 1

# Create an index.html so going to the root URL loads noVNC directly
cp /usr/share/novnc/vnc.html /usr/share/novnc/index.html

echo "Starting websockify for noVNC on port 5050"
websockify --web /usr/share/novnc/ 5050 localhost:5900 &

echo "Starting matchbox-window-manager"
DISPLAY=:0 matchbox-window-manager -use_titlebar yes &
sleep 1

# Clean up any leftover lock files from previous unclean shutdowns
echo "Cleaning up Chromium lock files..."
rm -f "$CHROME_DATA_DIR/SingletonLock"

echo "Starting socat proxy for Chrome DevTools on port 9223"
socat TCP-LISTEN:9223,fork,bind=0.0.0.0 TCP:127.0.0.1:9222 &
sleep 1

echo "Starting Chromium..."
DISPLAY=:0 chromium \
  --noerrdialogs \
  --disable-infobars \
  --no-first-run \
  --disable-pinch \
  --no-sandbox \
  --use-fake-ui-for-media-stream \
  --use-fake-device-for-media-stream \
  --disable-dev-shm-usage \
  --remote-debugging-port=9222 \
  --remote-debugging-address=0.0.0.0 \
  --remote-allow-origins="*" \
  --user-data-dir="$CHROME_DATA_DIR" \
  --window-size=1536,960 \
  --window-position=0,0 \
  "$LAUNCH_URL" &

# --enable-logging=stderr \rr \
# --v=1 \
# --v=1 \

# Wait for background processes
wait
