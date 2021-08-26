package packet

import (
	"bytes"
	"encoding/binary"
)

type Writer struct {
	Payload *bytes.Buffer
	Opcode  uint16
	Size    int32
	order   binary.ByteOrder
	err     error
}

func NewWriter(opcode uint16) *Writer {
	return &Writer{
		Opcode:  opcode,
		Payload: bytes.NewBuffer(nil),
		Size:    -1,
		order:   binary.LittleEndian,
	}
}

func NewWriterWithSize(opcode uint16, size uint32) *Writer {
	return &Writer{
		Opcode:  opcode,
		Payload: bytes.NewBuffer(make([]byte, 0, size)),
		Size:    int32(size),
		order:   binary.LittleEndian,
	}
}

func (w *Writer) ToPacket() *Packet {
	return &Packet{
		Opcode: w.Opcode,
		Size:   uint32(len(w.Payload.Bytes())),
		Data:   w.Payload.Bytes(),
	}
}

func (w *Writer) Uint8(v uint8) *Writer {
	if w.err != nil {
		return w
	}
	_, w.err = w.Payload.Write([]byte{v})
	return w
}

func (w *Writer) Bool(v bool) *Writer {
	var b byte
	if v == true {
		b = 1
	}

	if w.err != nil {
		return w
	}
	_, w.err = w.Payload.Write([]byte{b})
	return w
}

func (w *Writer) Uint16(v uint16) *Writer {
	if w.err != nil {
		return w
	}
	w.err = binary.Write(w.Payload, w.order, &v)
	return w
}

func (w *Writer) Int16(v int16) *Writer {
	if w.err != nil {
		return w
	}
	w.err = binary.Write(w.Payload, w.order, &v)
	return w
}

func (w *Writer) Uint32(v uint32) *Writer {
	if w.err != nil {
		return w
	}
	w.err = binary.Write(w.Payload, w.order, &v)
	return w
}

func (w *Writer) Int32(v int32) *Writer {
	if w.err != nil {
		return w
	}
	w.err = binary.Write(w.Payload, w.order, &v)
	return w
}

func (w *Writer) Uint64(v uint64) *Writer {
	if w.err != nil {
		return w
	}
	w.err = binary.Write(w.Payload, w.order, &v)
	return w
}

func (w *Writer) Int64(v int64) *Writer {
	if w.err != nil {
		return w
	}
	w.err = binary.Write(w.Payload, w.order, &v)
	return w
}

func (w *Writer) String(v string) *Writer {
	if w.err != nil {
		return w
	}
	w.err = binary.Write(w.Payload, w.order, append([]byte(v), 0))
	return w
}

func (w *Writer) Float32(v float32) *Writer {
	if w.err != nil {
		return w
	}
	w.err = binary.Write(w.Payload, w.order, &v)
	return w
}

func (w *Writer) Bytes(v []byte) *Writer {
	if w.err != nil {
		return w
	}
	w.err = binary.Write(w.Payload, w.order, &v)
	return w
}

func (w *Writer) SetByteOrder(order binary.ByteOrder) *Writer {
	w.order = order
	return w
}
