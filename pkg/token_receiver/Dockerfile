FROM --platform=$BUILDPLATFORM rust:1-slim as builder

ARG TARGETARCH

WORKDIR /usr/src/app
COPY Cargo.toml ./
COPY src ./src

# Install required dependencies for building iroh and cross compiling
RUN apt-get update && apt-get install -y \
    pkg-config \
    libssl-dev \
    gcc-aarch64-linux-gnu \
    libc6-dev-arm64-cross \
    && rm -rf /var/lib/apt/lists/*

# Map Docker's TARGETARCH to Rust's target triple
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        echo "aarch64-unknown-linux-gnu" > /rust_target.txt; \
    else \
        echo "x86_64-unknown-linux-gnu" > /rust_target.txt; \
    fi

RUN rustup target add $(cat /rust_target.txt)

# Set environment variables for cross-compilation (cargo looks for these)
ENV CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=aarch64-linux-gnu-gcc
ENV CC_aarch64_unknown_linux_gnu=aarch64-linux-gnu-gcc
ENV CXX_aarch64_unknown_linux_gnu=aarch64-linux-gnu-g++

RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/usr/src/app/target \
    RUST_TARGET=$(cat /rust_target.txt) && \
    cargo build --release --target $RUST_TARGET && \
    mkdir -p /usr/src/app/bin && \
    cp target/$RUST_TARGET/release/server /usr/src/app/bin/server

FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /usr/src/app/bin/server /app/server

ENV SERVER_ADDR=0.0.0.0:7070
EXPOSE 7070

CMD ["/app/server"]
