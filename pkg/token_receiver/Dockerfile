FROM rust:1-slim as builder

WORKDIR /usr/src/app
COPY Cargo.toml ./
COPY src ./src

# Install required dependencies for building iroh (like pkg-config, libssl-dev, etc if needed by rust/axum/iroh)
RUN apt-get update && apt-get install -y pkg-config libssl-dev && rm -rf /var/lib/apt/lists/*

RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/usr/src/app/target \
    cargo build --release && \
    mkdir -p /usr/src/app/bin && \
    cp target/release/server /usr/src/app/bin/server

FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /usr/src/app/bin/server /app/server

ENV SERVER_ADDR=0.0.0.0:7070
EXPOSE 7070

CMD ["/app/server"]
