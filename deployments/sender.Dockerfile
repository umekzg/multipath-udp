FROM golang:1.16

WORKDIR /go/src/app
COPY . .

RUN go build -o /sender cmd/sender/main.go

EXPOSE 1935/udp

CMD ["/sender"]