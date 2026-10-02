# Builds the TypeScript client and the CSS.
FROM node:22-alpine AS client

WORKDIR /app/src/ui/client

COPY src/ui/client/package.json src/ui/client/package-lock.json ./
RUN npm ci

# all of src, since Tailwind scans the templates for the classes they use
COPY src /app/src

# outputs to /app/src/ui/static/js and /app/src/ui/static/c
RUN npm run build && npm run build:css



# Generates the templates, and builds the server with the client and CSS embedded.
FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY src ./src
COPY --from=client /app/src/ui/static ./src/ui/static

# templ runs at the version in go.mod
RUN go run github.com/a-h/templ/cmd/templ generate

# nothing uses cgo, so this is a static binary
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /app/main .



FROM scratch

WORKDIR /app

COPY --from=builder /app/main ./main
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER 1000:1000

EXPOSE 9070

ENTRYPOINT ["/app/main"]
