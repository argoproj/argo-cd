FROM golang:1.22 AS builder

WORKDIR /src

COPY go.mod /src/go.mod
COPY go.sum /src/go.sum

RUN go mod download

# Perform the build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /dist/gitops ./agent


FROM alpine/git:v2.45.2
COPY --from=builder /dist/gitops /usr/local/bin/gitops
