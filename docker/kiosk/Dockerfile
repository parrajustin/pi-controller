FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    chromium \
    xserver-xorg-core \
    xserver-xorg-legacy \
    xserver-xorg-input-libinput \
    xserver-xorg-video-fbdev \
    x11-xserver-utils \
    xinit \
    unclutter \
    cec-utils \
    matchbox-window-manager \
    && rm -rf /var/lib/apt/lists/*

COPY start.sh /start.sh
RUN chmod +x /start.sh

ENV LAUNCH_URL "https://www.mountainviewmasoniclodge.com"
ENV LAUNCH_URL_HDMI "google.com"

CMD ["/start.sh"]
