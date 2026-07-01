#!/bin/bash

export LAUNCH_URL=${LAUNCH_URL:-"https://www.mountainviewmasoniclodge.com"}
export LAUNCH_URL_HDMI=${LAUNCH_URL_HDMI:-"https://www.google.com"}

# Create xinitrc to define X11 startup behavior
cat <<EOF > /root/.xinitrc
#!/bin/bash
xset -dpms
xset s noblank
xset s off
unclutter -idle 1 -root &

# Give X11 a moment to fully initialize outputs
sleep 2

echo "=== XRANDR DEBUG INFO ==="
xrandr
echo "========================="

POS1=\$(xrandr | grep " connected" | sed -n '1p' | grep -oE "\+[0-9]+\+[0-9]+" | sed 's/^+//; s/+/,/')
POS2=\$(xrandr | grep " connected" | sed -n '2p' | grep -oE "\+[0-9]+\+[0-9]+" | sed 's/^+//; s/+/,/')

if [ -n "\$POS2" ]; then
    chromium --kiosk --noerrdialogs --disable-infobars --no-first-run --disable-pinch --no-sandbox --disable-dev-shm-usage --ozone-platform=x11 --window-position="\$POS2" --user-data-dir=/tmp/chromium2 "$LAUNCH_URL_HDMI" &
fi

if [ -z "\$POS1" ]; then POS1="0,0"; fi

exec chromium --kiosk --noerrdialogs --disable-infobars --no-first-run --disable-pinch --no-sandbox --disable-dev-shm-usage --ozone-platform=x11 --window-position="\$POS1" "$LAUNCH_URL"
EOF

chmod +x /root/.xinitrc

# Start X11
echo "Starting X server and Chromium..."
exec xinit
