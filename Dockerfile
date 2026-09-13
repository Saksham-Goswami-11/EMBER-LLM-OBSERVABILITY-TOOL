# syntax=docker/dockerfile:1

# ---- frontend ----
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- backend (pure Go, no CGo — modernc.org/sqlite needs none) ----
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ember ./cmd/ember
RUN mkdir /data

# ---- runtime: one static binary, nothing else ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ember /ember
# distroless has no shell to chown at runtime, so a fresh named volume needs
# to inherit ownership from the image at build time — the nonroot user is
# uid/gid 65532 in this base image.
COPY --from=build --chown=65532:65532 /data /data
VOLUME ["/data"]
ENV EMBER_DB_PATH=/data/ember.db
EXPOSE 8080
ENTRYPOINT ["/ember"]
