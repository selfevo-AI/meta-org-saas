FROM rust:1.85-slim AS builder
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
COPY security-kernel/Cargo.toml ./security-kernel/Cargo.toml
COPY security-kernel/src ./security-kernel/src
RUN cargo build --release -p security-kernel

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/target/release/security-kernel /usr/local/bin/security-kernel
EXPOSE 8090
CMD ["security-kernel"]
