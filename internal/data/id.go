package data

import (
	"crypto/rand"
	"encoding/hex"
)

// NewId returns a GUID in the same shape as .NET's Guid.NewGuid().ToString(),
// which PortProxyGUI uses for the primary key of a rule.
func NewId() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	// Set the RFC 4122 version 4 and variant bits, as .NET does.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
