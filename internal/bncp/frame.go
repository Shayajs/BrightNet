// Package bncp implements BrightNet Core Protocol version 1.
// A frame is a fixed 12-byte header followed by a CBOR body.
package bncp

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

const (
	magic      = "BNCP"
	Version    = 1
	headerSize = 12
	MaxBody    = 2 << 20
)

const (
	TypeEnroll uint8 = iota + 1
	TypeEnrollAck
	TypeHeartbeat
	TypeHeartbeatAck
	TypeSnapshot
	TypeSnapshotAck
	TypeEvents
	TypeEventsAck
	TypeError uint8 = 255
)

var (
	ErrShort    = errors.New("trame BNCP trop courte")
	ErrMagic    = errors.New("trame BNCP inconnue")
	ErrVersion  = errors.New("version BNCP non prise en charge")
	ErrFlags    = errors.New("drapeaux BNCP non pris en charge")
	ErrTooLarge = errors.New("corps BNCP trop grand")
	ErrType     = errors.New("type BNCP inattendu")
)

type Frame struct {
	Version uint8
	Type    uint8
	Flags   uint16
	Body    []byte
}

var (
	enc cbor.EncMode
	dec cbor.DecMode
)

func init() {
	var err error
	enc, err = cbor.CanonicalEncOptions().EncMode()
	if err != nil {
		panic(err)
	}
	dec, err = cbor.DecOptions{
		MaxNestedLevels:  32,
		MaxArrayElements: 100000,
		MaxMapPairs:      100000,
		DefaultMapType:   reflect.TypeOf(map[string]any(nil)),
	}.DecMode()
	if err != nil {
		panic(err)
	}
}

func Write(w io.Writer, frame Frame) error {
	if frame.Version == 0 {
		frame.Version = Version
	}
	if len(frame.Body) > MaxBody {
		return ErrTooLarge
	}
	var head [headerSize]byte
	copy(head[:4], magic)
	head[4] = frame.Version
	head[5] = frame.Type
	binary.BigEndian.PutUint16(head[6:8], frame.Flags)
	binary.BigEndian.PutUint32(head[8:12], uint32(len(frame.Body)))
	if _, err := w.Write(head[:]); err != nil {
		return err
	}
	_, err := w.Write(frame.Body)
	return err
}

func Read(r io.Reader) (Frame, error) {
	var head [headerSize]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return Frame{}, ErrShort
		}
		return Frame{}, err
	}
	if string(head[:4]) != magic {
		return Frame{}, ErrMagic
	}
	if head[4] != Version {
		return Frame{}, ErrVersion
	}
	flags := binary.BigEndian.Uint16(head[6:8])
	if flags != 0 {
		return Frame{}, ErrFlags
	}
	n := binary.BigEndian.Uint32(head[8:12])
	if n > MaxBody {
		return Frame{}, ErrTooLarge
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return Frame{}, ErrShort
	}
	return Frame{Version: head[4], Type: head[5], Flags: flags, Body: body}, nil
}

// Marshal encodes value with the same names as JSON, then stores it as CBOR.
func Marshal(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return enc.Marshal(generic)
}

// Unmarshal decodes a CBOR body into dest using JSON field names.
func Unmarshal(body []byte, dest any) error {
	if len(body) == 0 {
		return nil
	}
	var generic any
	if err := dec.Unmarshal(body, &generic); err != nil {
		return err
	}
	raw, err := json.Marshal(generic)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

func (f Frame) String() string {
	return fmt.Sprintf("bncp v%d type=%d bytes=%d", f.Version, f.Type, len(f.Body))
}
