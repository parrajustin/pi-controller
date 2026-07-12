FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    chromium \
    xvfb \
    x11vnc \
    novnc \
    websockify \
    matchbox-window-manager \
    x11-xserver-utils \
    socat \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

COPY start.sh /start.sh
RUN chmod +x /start.sh

ENV LAUNCH_URL "https://meet.google.com"
ENV CHROME_DATA_DIR "/chrome-data"

EXPOSE 5050
EXPOSE 5051
EXPOSE 9223
EXPOSE 9225
EXPOSE 5900
EXPOSE 5901

CMD ["/start.sh"]
