# Stage 1: Build display_control_web
FROM node:20-alpine AS display_web_builder
WORKDIR /app
RUN npm install -g pnpm
COPY display_control_web/package.json display_control_web/pnpm-lock.yaml* ./
RUN pnpm install
COPY display_control_web/ ./
RUN pnpm run build-prod
# Remove non-serving files to prepare for a clean copy
RUN rm -rf node_modules src package.json pnpm-lock.yaml tsconfig.json pnpm-workspace.yaml

# Stage 2: Build meeting_control_web
FROM node:20-alpine AS meeting_web_builder
WORKDIR /app
RUN npm install -g pnpm
COPY meeting_control_web/package.json meeting_control_web/pnpm-lock.yaml* ./
RUN pnpm install
COPY meeting_control_web/ ./
RUN pnpm run build
# Remove non-serving files to prepare for a clean copy
RUN rm -rf node_modules src package.json pnpm-lock.yaml tsconfig.json eslint.config.mjs .prettierrc GEMINI.MD pnpm-workspace.yaml

# Stage 3: Build setup_web
FROM node:20-alpine AS setup_web_builder
WORKDIR /app
RUN npm install -g pnpm
COPY setup_web/package.json setup_web/pnpm-lock.yaml* ./
RUN pnpm install
COPY setup_web/ ./
RUN pnpm run build
# Remove non-serving files to prepare for a clean copy
RUN rm -rf node_modules src package.json pnpm-lock.yaml tsconfig.json

# Stage 4: Build Go binaries
FROM golang:alpine AS go_builder
WORKDIR /app
COPY . .
RUN go mod init github.com/parrajustin/pi-controller/pkg/lounge_display && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /build/display_server ./display_server/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /build/setup_server ./setup_server/main.go

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
COPY --from=go_builder /build/setup_server /app/setup_server

# Copy the rust binary
COPY --from=rust_builder /app/receiver /app/receiver

# Copy start script
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

# Copy setup_web
COPY --from=setup_web_builder /app /app/setup_web

# Copy the built display_control_web files to /app/public
COPY --from=display_web_builder /app /app/public

# Copy the built meeting_control_web files to a separate folder inside /app/public
COPY --from=meeting_web_builder /app /app/public/meeting_control_web

# The entrypoint launches the bash script which orchestrates setup and display servers
ENTRYPOINT ["/app/start.sh"]
