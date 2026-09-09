package murmur2

import (
	"hash"
	"testing"
)

func TestNew(t *testing.T) {
	h := New()
	if h == nil {
		t.Fatal("New returned nil")
	}

	// Verify the result satisfies hash.Hash32 (it should at compile
	// time, but a runtime check guards against future signature drift).
	var _ hash.Hash32 = h
}

func TestMurmur2CF_SizeAndBlockSize(t *testing.T) {
	m := &Murmur2CF{}

	if got := m.Size(); got != 4 {
		t.Errorf("Size() = %d, want 4", got)
	}

	if got := m.BlockSize(); got != 4 {
		t.Errorf("BlockSize() = %d, want 4", got)
	}
}

func TestMurmur2CF_RegressionPin(t *testing.T) {
	// "abc" has no whitespace, so the normalized buffer equals the
	// input. Murmur2 seed=1 over those three bytes produces the pinned
	// fingerprint — same value used by detect.go's getByteArrayHash
	// test (which calls into this implementation).
	m := &Murmur2CF{}
	if _, err := m.Write([]byte("abc")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got := m.Sum32(); got != 1621425345 {
		t.Errorf("Sum32 for \"abc\" = %d, want 1621425345", got)
	}
}

func TestMurmur2CF_StripsWhitespace(t *testing.T) {
	// The CF variant of Murmur2 ignores tab/LF/CR/space before
	// hashing, so these three inputs must produce identical
	// fingerprints. This is the property CurseForge fingerprint
	// matching relies on for cross-platform stability.
	makeHash := func(input string) uint32 {
		m := &Murmur2CF{}
		_, _ = m.Write([]byte(input))
		return m.Sum32()
	}

	a := makeHash("hello world")
	b := makeHash("hello\nworld")
	c := makeHash("hel l o\tw\rorld")

	if a == 0 {
		t.Fatal("hash of non-empty content should not be 0")
	}

	if a != b || a != c {
		t.Errorf("expected identical hashes after whitespace normalization; got %d / %d / %d", a, b, c)
	}
}

func TestMurmur2CF_Reset(t *testing.T) {
	m := &Murmur2CF{}
	_, _ = m.Write([]byte("hello"))
	before := m.Sum32()

	m.Reset()
	after := m.Sum32()

	if before == after {
		t.Errorf("Sum32 returned same value (%d) before and after Reset; expected different", before)
	}

	// After Reset the buffer is empty; the hash should match the
	// fresh-instance "empty input" hash.
	m2 := &Murmur2CF{}
	if m2.Sum32() != after {
		t.Errorf("post-Reset hash %d differs from fresh-instance empty-input hash %d", after, m2.Sum32())
	}
}

func TestMurmur2CF_WriteReturnsFullLength(t *testing.T) {
	// Even when bytes are stripped from the internal buffer, Write
	// must report having consumed every byte handed to it — that's
	// what the io.Writer contract requires.
	m := &Murmur2CF{}
	input := []byte("  hello\nworld  ")

	n, err := m.Write(input)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if n != len(input) {
		t.Errorf("Write returned n = %d, want %d (must report all bytes consumed even when some are stripped)",
			n, len(input))
	}
}

// Note: Murmur2CF.Sum(b) shares the LengthHasher.Sum bug — when b is
// non-nil and has cap < 4, it allocates and overwrites; otherwise it
// stomps the supplied prefix at offset 0 instead of appending. All
// current callers pass nil so the bug is latent. The tests above use
// Sum32 (which always calls Sum(nil)) to sidestep the issue. See the
// LengthHasher entry in .claude/TODO.md for the shared fix.
