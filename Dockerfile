FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /app/bin/bot ./cmd/bot
RUN go build -o /app/bin/scrapper ./cmd/scrapper

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/bin/ /app/bin/

COPY .env /app/

RUN apk --no-cache add ca-certificates

EXPOSE 8080 8081

CMD /app/bin/scrapper & /app/bin/bot 