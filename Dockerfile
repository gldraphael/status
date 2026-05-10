FROM golang:trixie AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/status .


FROM gcr.io/distroless/static-debian13

EXPOSE 8080
WORKDIR /app

COPY --from=builder /app/status /app/status

ENTRYPOINT ["/app/status"]
