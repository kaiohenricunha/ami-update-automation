# Stage 1: Build
FROM golang:1.26-alpine AS builder

ARG VERSION=dev
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /bootstrap \
    ./cmd/lambda/

# Stage 2: Lambda runtime
FROM public.ecr.aws/lambda/provided:al2023

COPY --from=builder /bootstrap /var/runtime/bootstrap

CMD ["bootstrap"]
