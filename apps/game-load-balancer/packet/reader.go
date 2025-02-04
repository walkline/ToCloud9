package packet

import (
	"bytes"
	"encoding/binary"
	"io"
)

type Reader struct {
	data  *bytes.Reader
	order binary.ByteOrder
	err   error
}

func NewReaderWithData(d []byte) *Reader {
	return &Reader{
		data:  bytes.NewReader(d),
		order: binary.LittleEndian,
		err:   nil,
	}
}

func (r *Reader) Int64() int64 {
	if r.err != nil {
		return 0
	}

	var result int64
	r.err = binary.Read(r.data, r.order, &result)
	return result
}

func (r *Reader) Uint64() uint64 {
	if r.err != nil {
		return 0
	}

	var result uint64
	r.err = binary.Read(r.data, r.order, &result)
	return result
}

func (r *Reader) Int32() int32 {
	if r.err != nil {
		return 0
	}

	var result int32
	r.err = binary.Read(r.data, r.order, &result)
	return result
}

func (r *Reader) Uint32() uint32 {
	if r.err != nil {
		return 0
	}

	var result uint32
	r.err = binary.Read(r.data, r.order, &result)
	return result
}

func (r *Reader) Uint16() uint16 {
	if r.err != nil {
		return 0
	}

	var result uint16
	r.err = binary.Read(r.data, r.order, &result)
	return result
}

func (r *Reader) Int16() int16 {
	if r.err != nil {
		return 0
	}

	var result int16
	r.err = binary.Read(r.data, r.order, &result)
	return result
}

func (r *Reader) Int8() int8 {
	if r.err != nil {
		return 0
	}

	var result int8
	r.err = binary.Read(r.data, r.order, &result)
	return result
}

func (r *Reader) Uint8() uint8 {
	if r.err != nil {
		return 0
	}

	var result uint8
	r.err = binary.Read(r.data, r.order, &result)
	return result
}

func (r *Reader) String() string {
	if r.err != nil {
		return ""
	}

	result := ""
	for {
		d := make([]byte, 1)
		n, err := r.data.Read(d)
		if err == io.EOF {
			break
		} else if err != nil {
			r.err = err
			return ""
		}

		if n == 0 || d[0] == 0 {
			break
		}
		result += string(d)
	}

	return result
}

func (r *Reader) Read(data interface{}) {
	if r.err != nil {
		return
	}

	r.err = binary.Read(r.data, r.order, data)
}

func (r *Reader) ReadGUID() uint64 {
	if r.err != nil {
		return 0
	}

	var guid uint64
	var guidMark uint8

	guidMark, err := r.data.ReadByte()
	if err != nil {
		r.err = err
		return 0
	}

	for i := 0; i < 8; i++ {
		if (guidMark & (1 << i)) != 0 {
			b, err := r.data.ReadByte()
			if err != nil {
				r.err = err
				return 0
			}

			// Read the next byte and update the guid
			bit := uint64(b)
			guid |= bit << (i * 8)
		}
	}

	return guid
}

func (r *Reader) Left() int {
	return r.data.Len()
}

func (r *Reader) Error() error {
	return r.err
}

func (r *Reader) RawReader() io.Reader {
	return r.data
}
