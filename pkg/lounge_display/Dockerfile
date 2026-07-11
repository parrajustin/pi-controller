# Stage 1: Build display_control_web
FROM --platform=$BUILDPLATFORM node:20-alpine AS display_web_builder
WORKDIR /app
RUN npm install -g pnpm
COPY --from=standard_ts_lib . /standard-ts-lib
COPY display_control_web/package.json display_control_web/pnpm-lock.yaml* ./
RUN pnpm install
COPY display_control_web/ ./
RUN pnpm run build
# Remove non-serving files to prepare for a clean copy
RUN rm -rf node_modules src package.json pnpm-lock.yaml tsconfig.json pnpm-workspace.yaml

# Stage 4: Build Go binaries
FROM --platform=$BUILDPLATFORM golang:alpine AS go_builder
ARG TARGETARCH
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod init github.com/parrajustin/pi-controller/pkg/lounge_display && go mod tidy
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -a -installsuffix cgo -o /build/display_server ./display_server

# Stage 6: Build token_receiver Rust binary
FROM --platform=$BUILDPLATFORM rust:1-slim AS rust_builder
ARG TARGETARCH
WORKDIR /app
# We use an additional build context for token_receiver
COPY --from=token_receiver . ./token_receiver/

RUN apt-get update && apt-get install -y \
    pkg-config \
    libssl-dev \
    gcc-aarch64-linux-gnu \
    libc6-dev-arm64-cross \
    && rm -rf /var/lib/apt/lists/*

RUN if [ "$TARGETARCH" = "arm64" ]; then \
        echo "aarch64-unknown-linux-gnu" > /rust_target.txt; \
    else \
        echo "x86_64-unknown-linux-gnu" > /rust_target.txt; \
    fi

RUN rustup target add $(cat /rust_target.txt)

ENV CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=aarch64-linux-gnu-gcc
ENV CC_aarch64_unknown_linux_gnu=aarch64-linux-gnu-gcc
ENV CXX_aarch64_unknown_linux_gnu=aarch64-linux-gnu-g++

RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/app/token_receiver/target \
    cd token_receiver && \
    RUST_TARGET=$(cat /rust_target.txt) && \
    cargo build --release --bin receiver --target $RUST_TARGET && \
    cp target/$RUST_TARGET/release/receiver /app/receiver

# Stage 7: Final image
FROM debian:12-slim
WORKDIR /oauth
WORKDIR /app

ENV OAUTH_DIR="/oauth"
ENV OAUTH_REDIRECT_URL="http://localhost:7070"

# Install ca-certificates required for OAuth / Google APIs over HTTPS
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# Copy the go binaries
COPY --from=go_builder /build/display_server /app/display_server

# Copy the rust binary
COPY --from=rust_builder /app/receiver /app/receiver

# Copy start script
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

# Copy the built display_control_web files to /app/public
COPY --from=display_web_builder /app /app/public

# The entrypoint launches the bash script which orchestrates setup and display servers
ENTRYPOINT ["/app/start.sh"]
