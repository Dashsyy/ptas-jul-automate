FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/bot ./cmd/bot

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/bot ./bot
ENV DB_PATH=/data/ptas.db
EXPOSE 8080
ENTRYPOINT ["./bot"]
