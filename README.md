# multipath-udp

A userspace implementation of multipath UDP. This project is heavily inspired by [srtla](https://github.com/BELABOX/srtla). The primary difference is that it is not coupled to the SRT protocol and is written in go for educational purposes. If you're looking for a real UDP tunnel, it would probably be better to use something like glorytun or mlvpn. This project has two core feature goals:

* Automatically register/deregister network interfaces.
* Schedule packets in a highly configurable manner, duplicating traffic if necessary.

Pretty much any other feature is ancillary.

## Usage

To run the sender:

```
go run cmd/sender/main.go
```

To run the receiver:

```
go run cmd/receiver/main.go
```
