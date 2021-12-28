package murmur2

import (
	"encoding/binary"
	"github.com/aviddiviner/go-murmur"
	"hash"
)

func New() hash.Hash32 {
	return &Murmur2CF{buf: make([]byte, 0)}
}

type Murmur2CF struct {
	// Can't be done incrementally, since it is seeded with the length of the input!
	buf []byte
}

func (m *Murmur2CF) Write(p []byte) (n int, err error) {
	for _, b := range p {
		if !isWhitespaceCharacter(b) {
			m.buf = append(m.buf, b)
		}
	}
	return len(p), nil
}

// CF modification: strips whitespace characters
func isWhitespaceCharacter(b byte) bool {
	return b == 9 || b == 10 || b == 13 || b == 32
}

func (m *Murmur2CF) Sum(b []byte) []byte {
	if b == nil {
		b = make([]byte, 4)
	}
	binary.BigEndian.PutUint32(b, murmur.MurmurHash2(m.buf, 1))
	return b
}

func (m *Murmur2CF) Reset() {
	m.buf = make([]byte, 0)
}

func (m *Murmur2CF) Size() int {
	return 4
}

func (m *Murmur2CF) BlockSize() int {
	return 4
}

func (m *Murmur2CF) Sum32() uint32 {
	return binary.BigEndian.Uint32(m.Sum(nil))
}
