FROM golang:1.23-alpine AS build
WORKDIR /src
COPY backend/go.mod ./
COPY backend/go.sum ./
COPY backend/vendor ./vendor
COPY backend/cmd ./cmd
RUN CGO_ENABLED=0 go build -mod=vendor -o /out/api-server ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/api-server ./api-server
RUN mkdir -p /app/data
EXPOSE 8080
CMD ["./api-server"]
