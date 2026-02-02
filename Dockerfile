FROM golang:1.24.12-bookworm AS builder

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
 && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN cd main && \
    go build -buildmode=plugin -o /out/llm.so ../mrapps/llm.go

RUN cd main && \
    CGO_ENABLED=1 GOOS=linux go build -o /out/mrcoordinator ./mrcoordinator.go

RUN cd main && \
    CGO_ENABLED=1 GOOS=linux go build -o /out/mrworker ./mrworker.go

FROM ubuntu:22.04

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/mrcoordinator /app/mrcoordinator
COPY --from=builder /out/mrworker /app/mrworker
COPY --from=builder /out/llm.so /app/llm.so

ENV PLUGIN_PATH=/app/llm.so

EXPOSE 8080

ENTRYPOINT ["/app/mrcoordinator"]

