# multipath-srt

This is a tool that sends an SRT stream over various network interfaces and receives them with a configurable jitter buffer. After the jitter buffer, the stream is sent to SRT.

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
