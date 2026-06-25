FROM node:20-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist pkg/api/web_dist
RUN CGO_ENABLED=0 go build -o assistant-agent main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /app/assistant-agent .
EXPOSE 8080
ENTRYPOINT ["./assistant-agent"]
CMD ["--serve"]
