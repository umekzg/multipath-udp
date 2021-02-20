FROM golang:1.16

WORKDIR /go/src/app
COPY . .

RUN go build -o /receiver cmd/receiver/main.go

EXPOSE 1985/udp

CMD ["/receiver"]