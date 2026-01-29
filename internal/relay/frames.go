package relay

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

const frameSizeLimit = 16 * 1024 * 1024

func WriteFrame(w io.Writer, payload []byte) error {
	length := uint32(len(payload))
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, length)
	if _, err := w.Write(header); err != nil {
		return err
	}
	if length == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

func ReadFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)
	if length == 0 {
		return nil, nil
	}
	if length > frameSizeLimit {
		return nil, errors.New("frame too large")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func WriteJSON(w io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return WriteFrame(w, payload)
}

func ReadJSON(r io.Reader, target any) error {
	payload, err := ReadFrame(r)
	if err != nil {
		return err
	}
	if payload == nil {
		return io.EOF
	}
	return json.Unmarshal(payload, target)
}

type FrameReader struct {
	r         io.Reader
	remaining int
	done      bool
}

func NewFrameReader(r io.Reader) *FrameReader {
	return &FrameReader{r: r}
}

func (fr *FrameReader) Read(p []byte) (int, error) {
	if fr.done {
		return 0, io.EOF
	}
	if fr.remaining == 0 {
		header := make([]byte, 4)
		if _, err := io.ReadFull(fr.r, header); err != nil {
			return 0, err
		}
		length := int(binary.BigEndian.Uint32(header))
		if length == 0 {
			fr.done = true
			return 0, io.EOF
		}
		if length > frameSizeLimit {
			return 0, errors.New("frame too large")
		}
		fr.remaining = length
	}

	if len(p) == 0 {
		return 0, nil
	}

	toRead := fr.remaining
	if toRead > len(p) {
		toRead = len(p)
	}

	n, err := fr.r.Read(p[:toRead])
	if n > 0 {
		fr.remaining -= n
	}
	return n, err
}

type FrameWriter struct {
	w io.Writer
}

func NewFrameWriter(w io.Writer) *FrameWriter {
	return &FrameWriter{w: w}
}

func (fw *FrameWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if err := WriteFrame(fw.w, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (fw *FrameWriter) Close() error {
	return WriteFrame(fw.w, nil)
}
