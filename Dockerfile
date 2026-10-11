FROM golang:1.27.1-alpine

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o agent .

CMD ["./agent"]
