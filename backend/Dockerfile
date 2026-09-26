FROM golang:1.22-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && go build -trimpath -ldflags="-s -w" -o /out/seed ./seed

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/api /out/seed ./
USER app
EXPOSE 8080
CMD ["/app/api"]
