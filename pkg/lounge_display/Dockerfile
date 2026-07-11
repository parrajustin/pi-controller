# Stage 1: Build display_control_web
FROM node:20-alpine AS display_web_builder
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
FROM golang:alpine AS go_builder
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod init github.com/parrajustin/pi-controller/pkg/lounge_display && go mod tidy
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /build/display_server ./display_server

# Stage 6: Build token_receiver Rust binary
FROM rust:slim AS rust_builder
WORKDIR /app
# We use an additional build context for token_receiver
COPY --from=token_receiver . ./token_receiver/
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/app/token_receiver/target \
    cd token_receiver && cargo build --release --bin receiver && \
    cp target/release/receiver /app/receiver

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
