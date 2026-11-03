package runner

import "encoding/hex"

// hexEncode encodes bytes as lowercase hexadecimal.
func hexEncode(b []byte) string { return hex.EncodeToString(b) }
