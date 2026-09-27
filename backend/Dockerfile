FROM golang:1.24-alpine AS build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/campusclaw ./cmd/server

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app && mkdir -p /data/uploads && chown -R app:app /data
COPY --from=build /out/campusclaw /usr/local/bin/campusclaw
USER app
EXPOSE 8080
CMD ["/usr/local/bin/campusclaw"]
