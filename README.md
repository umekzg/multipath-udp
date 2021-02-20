# multipath-udp

A userspace implementation of multipath UDP. This project is heavily inspired by [srtla](https://github.com/BELABOX/srtla). The primary difference is that it is not coupled to the SRT protocol and is written in go for educational purposes.

## Usage

To run the sender:

```
go run cmd/sender/main.go
```

To run the receiver:

```
go run cmd/receiver/main.go
```
