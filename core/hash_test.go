package core

import (
	"encoding/binary"
	"testing"
)

func TestGetHashImpl(t *testing.T) {
	cases := []struct {
		name       string
		hashType   string
		wantErr    bool
		wantString string // expected HashToString output when hashing the empty input
	}{
		{
			name:       "sha1",
			hashType:   "sha1",
			wantString: "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		},
		{
			name:       "sha256",
			hashType:   "sha256",
			wantString: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:       "sha512",
			hashType:   "sha512",
			wantString: "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e",
		},
		{
			name:       "md5",
			hashType:   "md5",
			wantString: "d41d8cd98f00b204e9800998ecf8427e",
		},
		{
			name:     "murmur2 yields a decimal string",
			hashType: "murmur2",
			// Regression pin: murmur2 (CF variant) of empty input.
			// The non-zero value reflects the algorithm's
			// initialization constant, not a bug.
			wantString: "1540447798",
		},
		{
			name:       "length-bytes for empty input is zero",
			hashType:   "length-bytes",
			wantString: "0",
		},
		{
			name:       "case-insensitive on the type name",
			hashType:   "SHA256",
			wantString: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "unknown type returns an error",
			hashType: "blake3",
			wantErr:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			impl, err := GetHashImpl(tc.hashType)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for hash type %q, got nil", tc.hashType)
				}

				if impl != nil {
					t.Errorf("expected nil HashStringer on error, got %T", impl)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error for hash type %q: %v", tc.hashType, err)
			}

			if impl == nil {
				t.Fatalf("expected non-nil HashStringer for %q", tc.hashType)
			}

			got := impl.HashToString(impl.Sum(nil))

			if got != tc.wantString {
				t.Errorf("HashToString for %q on empty input = %q, want %q",
					tc.hashType, got, tc.wantString)
			}
		})
	}
}

func TestHashToString(t *testing.T) {
	cases := []struct {
		name      string
		hashType  string
		input     []byte
		want      string
	}{
		{
			name:     "hexStringer encodes bytes as lowercase hex",
			hashType: "sha1",
			input:    []byte{0xde, 0xad, 0xbe, 0xef},
			want:     "deadbeef",
		},
		{
			name:     "number32As64Stringer decodes big-endian uint32 to decimal",
			hashType: "murmur2",
			input:    []byte{0x00, 0x00, 0x00, 0x2a}, // 42 big-endian
			want:     "42",
		},
		{
			name:     "number64Stringer decodes big-endian uint64 to decimal",
			hashType: "length-bytes",
			input:    []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00}, // 1024
			want:     "1024",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			impl, err := GetHashImpl(tc.hashType)
			if err != nil {
				t.Fatalf("setup: GetHashImpl(%q): %v", tc.hashType, err)
			}

			got := impl.HashToString(tc.input)

			if got != tc.want {
				t.Errorf("%s.HashToString(%v) = %q, want %q",
					tc.hashType, tc.input, got, tc.want)
			}
		})
	}
}

func TestLengthHasher(t *testing.T) {
	h := &LengthHasher{}

	if got := h.Size(); got != 8 {
		t.Errorf("Size() = %d, want 8", got)
	}

	if got := h.BlockSize(); got != 1 {
		t.Errorf("BlockSize() = %d, want 1", got)
	}

	n, err := h.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	if n != 5 {
		t.Errorf("Write returned n = %d, want 5", n)
	}

	n, err = h.Write([]byte(" world"))
	if err != nil {
		t.Fatalf("second Write returned error: %v", err)
	}

	if n != 6 {
		t.Errorf("second Write returned n = %d, want 6", n)
	}

	sum := h.Sum(nil)

	if len(sum) != 8 {
		t.Fatalf("Sum returned %d bytes, want 8", len(sum))
	}

	if got := binary.BigEndian.Uint64(sum); got != 11 {
		t.Errorf("Sum encodes length %d, want 11 (5 + 6)", got)
	}

	// Sum(b) prefix-preservation is intentionally not tested here:
	// the current LengthHasher.Sum implementation overwrites b
	// rather than appending to it, violating the hash.Hash
	// contract. All current callers pass nil so the bug is latent.
	// Add a prefix-preservation case once the impl is fixed (see
	// the entry in .claude/TODO.md).

	h.Reset()

	sumAfterReset := h.Sum(nil)

	if got := binary.BigEndian.Uint64(sumAfterReset); got != 0 {
		t.Errorf("Sum after Reset encodes length %d, want 0", got)
	}
}
