package xbox

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"math/big"
	"net/http"
	"time"
)

// sign signs the request passed containing the body passed. It signs the request using the ECDSA private key
// passed. If the request has a 'ProofKey' field in the Properties field, that key must be passed here.
func sign(request *http.Request, body []byte, key *ecdsa.PrivateKey) {
	currentTime := windowsTimestamp()
	hash := sha256.New()

	// Signature policy version (0, 0, 0, 1) + 0 byte.
	buf := bytes.NewBuffer([]byte{0, 0, 0, 1, 0})
	// Timestamp + 0 byte.
	_ = binary.Write(buf, binary.BigEndian, currentTime)
	buf.Write([]byte{0})
	hash.Write(buf.Bytes())

	// HTTP method, generally POST + 0 byte.
	hash.Write([]byte("POST"))
	hash.Write([]byte{0})
	// Request uri path + raw query + 0 byte.
	path := request.URL.Path
	if rq := request.URL.RawQuery; rq != "" {
		path += "?" + rq
	}
	hash.Write([]byte(path))
	hash.Write([]byte{0})

	// Authorization header if present, otherwise an empty string + 0 byte.
	hash.Write([]byte(request.Header.Get("Authorization")))
	hash.Write([]byte{0})

	// Body data (only up to a certain limit, but this limit is practically never reached) + 0 byte.
	hash.Write(body)
	hash.Write([]byte{0})

	// Sign the checksum produced, and combine the 'r' and 's' into a single signature.
	// Encode r and s as 32-byte, zero-padded big-endian values so the P-256 signature is always exactly 64 bytes long.
	r, s, _ := ecdsa.Sign(rand.Reader, key, hash.Sum(nil))
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])

	// The signature begins with 12 bytes, the first being the signature policy version (0, 0, 0, 1) again,
	// and the other 8 the timestamp again.
	buf = bytes.NewBuffer([]byte{0, 0, 0, 1})
	_ = binary.Write(buf, binary.BigEndian, currentTime)

	// Append the signature to the other 12 bytes, and encode the signature with standard base64 encoding.
	sig := append(buf.Bytes(), signature...)
	request.Header.Set("Signature", base64.StdEncoding.EncodeToString(sig))
}

// windowsTimestamp returns a Windows specific timestamp. It has a certain offset from Unix time which must be
// accounted for.
func windowsTimestamp() int64 {
	return (time.Now().Unix() + 11644473600) * 10000000
}

// padTo32Bytes converts a big.Int into a fixed 32-byte, zero-padded slice.
// This is used to ensure that the X and Y coordinates of the ECDSA public key are always 32 bytes long,
// because big.Int.Bytes() returns a minimal encoding which may sometimes be less than 32 bytes.
func padTo32Bytes(b *big.Int) []byte {
	out := make([]byte, 32)
	b.FillBytes(out)
	return out
}
