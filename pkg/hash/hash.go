package hash

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"encoding/hex"
	"sort"
	"strings"
)

type Hasher interface {
	Hash() string
}

type String string

func (s String) Hash() string {
	h := sha1.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

type collator []Hasher

func Collate(hashers ...Hasher) Hasher {
	return collator(hashers)
}

// Hashes a set of Hashers to the same hash irrespective of order.
func (c collator) Hash() string {
	hashes := make([]string, len(c))
	for _, hasher := range c {
		hashes = append(hashes, hasher.Hash())
	}
	sort.Strings(hashes)
	h := sha1.New()
	h.Write([]byte(strings.Join(hashes, "")))
	return hex.EncodeToString(h.Sum(nil))
}

type Struct struct {
	*gob.Encoder 
	buf *bytes.Buffer
}

func NewStruct() *Struct {
	buf := &bytes.Buffer{}
	return &Struct{gob.NewEncoder(buf), buf}
}

// Hashes a struct by gob encoding it, then hashing the encoded bytes. Only the exported fields of the struct will be
// used during computation of the hash.
func (s *Struct) Hash() string {
	h := sha1.New()
	h.Write(s.buf.Bytes())
	return hex.EncodeToString(h.Sum(nil))
}