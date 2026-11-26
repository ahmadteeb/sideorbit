# Build context is the repository root: docker build -f server/Dockerfile .
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/orbit-server .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/orbit-server /usr/local/bin/orbit-server
COPY LICENSE NOTICE /licenses/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/orbit-server"]
