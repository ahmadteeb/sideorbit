# Build context is the repository root: docker build -f server/Dockerfile .
FROM node:26-alpine AS web
WORKDIR /src/web
RUN npm install --global pnpm@12.17.0
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web /src/web/build/ ./internal/web/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/orbit-server .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/orbit-server /usr/local/bin/orbit-server
COPY LICENSE NOTICE /licenses/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/orbit-server"]
