FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
COPY ./manager ./manager
COPY ./utils ./utils
WORKDIR /app/manager
RUN go mod download
RUN go build -o . ./

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/manager .
EXPOSE 8080
CMD ["./TaskOneManager"]