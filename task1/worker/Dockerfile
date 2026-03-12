FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
COPY ./worker ./worker
COPY ./utils ./utils
WORKDIR /app/worker
RUN go mod download
RUN go build -o worker ./

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/worker .
EXPOSE 8081
CMD ["./worker"]