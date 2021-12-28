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
func GetHashImpl(hashType string) (hash.Hash, HashStringer, error) {
	switch strings.ToLower(hashType) {
	case "sha1":
		return sha1.New(), hexStringer{}, nil
	case "sha256":
		return sha256.New(), hexStringer{}, nil
	case "sha512":
		return sha512.New(), hexStringer{}, nil
	case "md5":
		return md5.New(), hexStringer{}, nil
	case "murmur2":
		return murmur2.New(), numberStringer{}, nil
	}
	return nil, nil, fmt.Errorf("hash implementation %s not found", hashType)
}

type HashStringer interface {
	HashToString([]byte) string
}

type hexStringer struct{}

func (hexStringer) HashToString(data []byte) string {
	return hex.EncodeToString(data)
}

type numberStringer struct{}

func (numberStringer) HashToString(data []byte) string {
	return strconv.FormatUint(uint64(binary.BigEndian.Uint32(data)), 10)
}
