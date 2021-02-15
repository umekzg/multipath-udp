FROM golang:1.15

WORKDIR /go/src/app
COPY . .

RUN go mod download
RUN go build -o ./receiver cmd/receiver/main.go

CMD ["/go/src/app/receiver"]