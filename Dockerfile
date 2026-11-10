# One image that contains both Go programs (api and listener).
# docker-compose.yml chooses which one to run with `command:`.

# Stage 1: compile. This stage has the Go toolchain and is thrown away after.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
# CGO_ENABLED=0 makes self-contained binaries that run on plain Alpine.
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -o /out/listener ./cmd/listener

# Stage 2: the small image that actually runs.
FROM alpine:3.20
# Do not run as root inside the container.
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/listener /usr/local/bin/listener
CMD ["api"]
