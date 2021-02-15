FROM golang:1.15

WORKDIR /go/src/app
COPY . .

ENV INPUT_ADDRESS ":1936"
ENV OUTPUT_ADDRESS "rtmp:1935"

RUN go mod download
RUN go build -o ./receiver cmd/receiver/main.go

CMD ["/go/src/app/receiver", "-i", "${INPUT}", "-o", "${OUTPUT}"]