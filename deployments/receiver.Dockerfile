FROM golang:1.15

WORKDIR /srt
RUN apt update && apt install -y tcl cmake libssl-dev
RUN git clone https://github.com/Haivision/srt
RUN cd srt && ./configure && make && make install

WORKDIR /go/src/app
COPY . .

RUN go mod download
RUN go build -o ./receiver cmd/receiver/main.go

ENV LD_LIBRARY_PATH /usr/local/lib

EXPOSE 1985/udp

CMD ["/go/src/app/receiver"]