FROM golang:1.24.1 AS builder

WORKDIR /app

# Download the Tailwind CSS Linux executable
RUN curl -L -o /usr/local/bin/tailwind \
    https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.7/tailwindcss-linux-x64 && \
    chmod +x /usr/local/bin/tailwind


COPY go.mod go.sum Makefile ./

RUN make download-tools install-go

COPY ./.git ./.git
COPY ./src ./src
COPY main.go ./

RUN ls -la && make build-site



# FROM alpine:latest
FROM scratch

WORKDIR /app

ENV DEBUG=false

COPY --from=builder /app/main.o ./main
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER 1000:1000

EXPOSE 9070

ENTRYPOINT ["/app/main"]

