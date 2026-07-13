#!/bin/bash

export LAUNCH_URL=${LAUNCH_URL:-"https://www.mountainviewmasoniclodge.com"}
export LAUNCH_URL_HDMI=${LAUNCH_URL_HDMI:-"https://www.google.com"}
export CHROME_DATA_DIR=${CHROME_DATA_DIR:-"/chrome-data"}

# Clean up any leftover lock files from sudden crashes so chromium can restart
rm -f $CHROME_DATA_DIR/dsi/Singleton* $CHROME_DATA_DIR/hdmi/Singleton*

# Dynamically generate xorg.conf based on connected DRM devices
echo "=== Detecting Connected DRM Displays ==="
CARDS=()
for dir in /sys/class/drm/card?-*; do
    if [ -f "$dir/status" ] && grep -q "^connected$" "$dir/status"; then
        # Extract just the "cardX" part
        card_name=$(echo "$dir" | grep -oE "card[0-9]+" | head -n1)
        # Add to array if not already present
        if [[ ! " ${CARDS[@]} " =~ " ${card_name} " ]]; then
            CARDS+=("$card_name")
            echo "Found connected display on $card_name ($dir)"
        fi
    fi
done

NUM_CARDS=${#CARDS[@]}
echo "Total connected cards: $NUM_CARDS"

if [ "$NUM_CARDS" -gt 0 ]; then
    XORG_CONF="/etc/X11/xorg.conf"
    echo "" > "$XORG_CONF"
    
    # Create Device and Screen sections for each card
    for i in "${!CARDS[@]}"; do
        card="${CARDS[$i]}"
        cat <<EOF >> "$XORG_CONF"
Section "Device"
    Identifier "Device$i"
    Driver "modesetting"
    Option "kmsdev" "/dev/dri/$card"
EndSection

Section "Screen"
    Identifier "Screen$i"
    Device "Device$i"
EndSection

EOF
    done

    # Create ServerLayout to tie them all together
    echo "Section \"ServerLayout\"" >> "$XORG_CONF"
    echo "    Identifier \"Layout0\"" >> "$XORG_CONF"
    for i in "${!CARDS[@]}"; do
        if [ $i -eq 0 ]; then
            echo "    Screen $i \"Screen$i\"" >> "$XORG_CONF"
        else
            prev=$((i-1))
            echo "    Screen $i \"Screen$i\" RightOf \"Screen$prev\"" >> "$XORG_CONF"
        fi
    done
    echo "EndSection" >> "$XORG_CONF"
    
    echo "=== Generated xorg.conf ==="
    cat "$XORG_CONF"
    echo "==========================="
fi

cat <<EOF > /root/.xinitrc
#!/bin/bash
xset -dpms
xset s noblank
xset s off
unclutter -idle 1 -root &

sleep 2

# Helper to launch chromium on a specific X screen
launch_chrome() {
    local DISP=":\$1"
    local URL="\$2"
    local DIR="\$3"
    local EXTRA_ARGS="\$4"
    
    echo "=== Xrandr Debug for DISPLAY=\$DISP ==="
    DISPLAY="\$DISP" xrandr || true
    echo "======================================="

    # Start a lightweight window manager on this screen.
    # This guarantees the window maximizes to the full physical screen, 
    # overriding any weird scaling bugs.
    DISPLAY="\$DISP" matchbox-window-manager -use_titlebar no &

    echo "Launching Chromium on DISPLAY=\$DISP with URL \$URL"
    DISPLAY="\$DISP" chromium \\
      --kiosk \\
      --noerrdialogs \\
      --disable-infobars \\
      --no-first-run \\
      --disable-pinch \\
      --no-sandbox \\
      --disable-dev-shm-usage \\
      --ozone-platform=x11 \\
      --user-data-dir="\$DIR" \\
      \$EXTRA_ARGS \\
      "\$URL" &
}

# Launch on the first display (usually DSI / card0)
# DSI is 800x480, normal scaling
launch_chrome "0.0" "\$LAUNCH_URL" "\$CHROME_DATA_DIR/dsi" "--force-device-scale-factor=1 --remote-debugging-port=9223 --remote-debugging-address=0.0.0.0 --remote-allow-origins=*"

# If we found multiple cards (like the HDMI on card2), launch on the second display!
if [ "$NUM_CARDS" -ge 2 ]; then
    # HDMI is 4K, force 3x scaling so text is readable
    launch_chrome "0.1" "\$LAUNCH_URL_HDMI" "\$CHROME_DATA_DIR/hdmi" "--force-device-scale-factor=2.5 --remote-debugging-port=9222 --remote-debugging-address=0.0.0.0 --remote-allow-origins=*"
fi

# Wait for background processes so X doesn't exit immediately
wait
EOF

chmod +x /root/.xinitrc

# Start X11
echo "Starting X server and Chromium..."
exec xinit -- -nocursor
