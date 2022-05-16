package core

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/packwiz/packwiz/curseforge/murmur2"
	"hash"
	"strconv"
	"strings"
)

// GetHashImpl gets an implementation of hash.Hash for the given hash type string
func GetHashImpl(hashType string) (HashStringer, error) {
	switch strings.ToLower(hashType) {
	case "sha1":
		return hexStringer{sha1.New()}, nil
	case "sha256":
		return hexStringer{sha256.New()}, nil
	case "sha512":
		return hexStringer{sha512.New()}, nil
	case "md5":
		return hexStringer{md5.New()}, nil
	case "murmur2": // TODO: change to something indicating that this is the CF variant
		return number32As64Stringer{murmur2.New()}, nil
	case "length-bytes": // TODO: only used internally for now; should not be saved
		return number64Stringer{&LengthHasher{}}, nil
	}
	return nil, fmt.Errorf("hash implementation %s not found", hashType)
}

type HashStringer interface {
	hash.Hash
	HashToString([]byte) string
}

type hexStringer struct {
	hash.Hash
}

func (hexStringer) HashToString(data []byte) string {
	return hex.EncodeToString(data)
}

type number32As64Stringer struct {
	hash.Hash
}

func (number32As64Stringer) HashToString(data []byte) string {
	return strconv.FormatUint(uint64(binary.BigEndian.Uint32(data)), 10)
}

type number64Stringer struct {
	hash.Hash
}

func (number64Stringer) HashToString(data []byte) string {
	return strconv.FormatUint(binary.BigEndian.Uint64(data), 10)
}

type LengthHasher struct {
	length uint64
}

func (h *LengthHasher) Write(p []byte) (n int, err error) {
	h.length += uint64(len(p))
	return len(p), nil
}

func (h *LengthHasher) Sum(b []byte) []byte {
	ext := append(b, make([]byte, 8)...)
	binary.BigEndian.PutUint64(ext, h.length)
	return ext
}

func (h *LengthHasher) Size() int {
	return 8
}

func (h *LengthHasher) BlockSize() int {
	return 1
}

func (h *LengthHasher) Reset() {
	h.length = 0
}
