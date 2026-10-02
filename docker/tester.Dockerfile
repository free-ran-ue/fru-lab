# -------------------------------
# Stage 1: Build fru-tester
# -------------------------------
FROM golang:1.26.2 AS builder
WORKDIR /src/tester

COPY tester/go.mod tester/go.sum ./
RUN go mod download

COPY tester/ .
RUN CGO_ENABLED=0 go build -o /out/fru-tester .

# -------------------------------
# Stage 2: Runtime
# -------------------------------
FROM debian:bookworm-slim
WORKDIR /frutester

COPY --from=builder /out/fru-tester ./fru-tester
