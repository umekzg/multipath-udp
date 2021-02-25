package protocol

import (
	"bytes"
	"encoding/gob"
)

// Handshake represents the initial handshake between a sender and the receiver.
type Handshake struct {
	Sender  string
	Session []byte
}

// NewHandshake creates a new handshake.
func NewHandshake(sender string, session []byte) *Handshake {
	return &Handshake{
		Sender:  sender,
		Session: session,
	}
}

// Serialize a handshake to bytes.
func (h Handshake) Serialize() ([]byte, error) {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(h); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Deserialize a handshake from bytes.
func Deserialize(buf []byte) (*Handshake, error) {
	var h Handshake
	if err := gob.NewDecoder(bytes.NewReader(buf)).Decode(&h); err != nil {
		return nil, err
	}
	return &h, nil
}
