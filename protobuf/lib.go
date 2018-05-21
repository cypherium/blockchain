package stream

import (
	"encoding/binary"
	"io"
	"sync"

	"github.com/gogo/protobuf/proto"
)

type Message interface {
	proto.Marshaler
	proto.Unmarshaler
	proto.Sizer
}

type LockableWriter interface {
	sync.Locker
	io.Writer
}

const bytesForSize = 8

// Write writes the marshalled message into the writer
func Write(w io.Writer, m Message) (err error) {
	messageBytes, err := m.Marshal()

	if err == nil {
		size := m.Size()
		sizeBytes := make([]byte, bytesForSize)
		binary.BigEndian.PutUint64(sizeBytes, uint64(size))

		w.Write(sizeBytes)
		w.Write(messageBytes)
	}

	return
}

// WriteWithLock is just like Write, but will lock on write
func WriteWithLock(w LockableWriter, m Message) (err error) {
	messageBytes, err := m.Marshal()

	if err == nil {
		size := m.Size()
		sizeBytes := make([]byte, bytesForSize)
		binary.BigEndian.PutUint64(sizeBytes, uint64(size))

		w.Lock()
		defer w.Unlock()
		w.Write(sizeBytes)
		w.Write(messageBytes)
	}

	return
}

// Read reads the message from the reader
func Read(r io.Reader, m Message) (err error) {
	sizeBytes := make([]byte, bytesForSize)

	if _, err = io.ReadFull(r, sizeBytes); err == nil {
		size := binary.BigEndian.Uint64(sizeBytes)
		messageBytes := make([]byte, size)

		if _, err = io.ReadFull(r, messageBytes); err == nil {
			m.Unmarshal(messageBytes)
		}
	}

	return
}
