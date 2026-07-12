#!/bin/bash

# Clean up any leftover X server locks\nrm -f /tmp/.X0-lock /tmp/.X1-lock\nrm -rf /tmp/.X11-unix

export LAUNCH_URL=${LAUNCH_URL:-"https://meet.google.com"}
export CHROME_DATA_DIR=${CHROME_DATA_DIR:-"/chrome-data"}
export LAUNCH_URL_2=${LAUNCH_URL_2:-"http://localhost:8080"}
export CHROME_DATA_DIR_2=${CHROME_DATA_DIR_2:-"/chrome-data-2"}

echo "Starting Xvfb on :0 and :1"
Xvfb :0 -screen 0 1536x960x24 -listen tcp -ac &
Xvfb :1 -screen 0 800x600x24 -listen tcp -ac &
sleep 1

echo "Starting x11vnc for both displays"
x11vnc -display :0 -nopw -listen localhost -xkb -forever &
x11vnc -display :1 -nopw -listen localhost -xkb -forever -rfbport 5901 &
sleep 1

# Create an index.html so going to the root URL loads noVNC directly
cp /usr/share/novnc/vnc.html /usr/share/novnc/index.html

echo "Starting websockify for noVNC on ports 5050 and 5051"
websockify --web /usr/share/novnc/ 5050 localhost:5900 &
websockify --web /usr/share/novnc/ 5051 localhost:5901 &

echo "Starting matchbox-window-manager for both displays"
DISPLAY=:0 matchbox-window-manager -use_titlebar yes &
DISPLAY=:1 matchbox-window-manager -use_titlebar yes &
sleep 1

# Clean up any leftover lock files from previous unclean shutdowns
echo "Cleaning up Chromium lock files..."
rm -f "$CHROME_DATA_DIR/SingletonLock"
rm -f "$CHROME_DATA_DIR_2/SingletonLock"
rm -f "$CHROME_DATA_DIR/SingletonCookie"
rm -f "$CHROME_DATA_DIR/SingletonSocket"
rm -f "$CHROME_DATA_DIR_2/SingletonCookie"
rm -f "$CHROME_DATA_DIR_2/SingletonSocket"

echo "Starting socat proxies for Chrome DevTools on ports 9223 and 9225"
socat TCP-LISTEN:9223,fork,bind=0.0.0.0 TCP:127.0.0.1:9222 &
socat TCP-LISTEN:9225,fork,bind=0.0.0.0 TCP:127.0.0.1:9224 &
sleep 1

echo "Starting Chromium for display 1..."
DISPLAY=:0 chromium \
  --noerrdialogs \
  --disable-infobars \
  --no-first-run \
  --disable-pinch \
  --disable-gpu \
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
  "$LAUNCH_URL" > /dev/null 2>&1 &

echo "Starting Chromium for display 2..."
DISPLAY=:1 chromium \
  --noerrdialogs \
  --disable-infobars \
  --no-first-run \
  --disable-pinch \
  --disable-gpu \
  --no-sandbox \
  --use-fake-ui-for-media-stream \
  --use-fake-device-for-media-stream \
  --disable-dev-shm-usage \
  --remote-debugging-port=9224 \
  --remote-debugging-address=0.0.0.0 \
  --remote-allow-origins="*" \
  --user-data-dir="$CHROME_DATA_DIR_2" \
  --window-size=800,600 \
  --window-position=0,0 \
  "$LAUNCH_URL_2" > "$CHROME_DATA_DIR_2/chromium.log" 2>&1 &

if [ "$RECORD_SCREENS" = "true" ]; then
  echo "Starting screen recording for Display 1 and 2..."
  mkdir -p /recordings
  # Record Display 1 (segment every 60 seconds)
  ffmpeg -y -f x11grab -video_size 1536x960 -i :0 -codec:v libx264 -preset ultrafast -pix_fmt yuv420p -f segment -segment_time 60 -reset_timestamps 1 /recordings/display1_%03d.mp4 > /dev/null 2>&1 &
  # Record Display 2 (segment every 60 seconds)
  ffmpeg -y -f x11grab -video_size 800x600 -i :1 -codec:v libx264 -preset ultrafast -pix_fmt yuv420p -f segment -segment_time 60 -reset_timestamps 1 /recordings/display2_%03d.mp4 > /dev/null 2>&1 &
fi

# Wait for background processes
wait
