# multipath-rtp

A tool that ingests RTMP, schedules it over multiple interfaces, and spits it out as an SRT stream.

## Usage

To run the sender:

```
go run cmd/sender/main.go
```

To run the receiver:

```
go run cmd/receiver/main.go
```

## Design

### Demuxer

- Read from librtmp, depacketize
- Packetize, add sequence number packet header
- Loop through `InterfaceSet` on demuxer
- Schedule and send through interfaces

- Read from `InterfaceSet` bandwidth rates

### Muxer

- Receive connection with `sessionID`
  - If new connection, open SRT connection with `sessionID`
- Respond with handshake
- Read UDP packets until error
- Depacketize, read sequence number
- Reorder, deduplicate
- Write to SRT, ignoring dropped packets (receiver will handle)

### InterfaceSet

```
NewInterfaceSet(sessionID []byte)  # for handshaking
AddInterface()
RemoveInterface()
```
