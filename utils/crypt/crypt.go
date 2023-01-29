package crypt

import (
	"bytes"
	_ "embed"
	"io"
	"time"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

//go:embed key.gpg
var key_gpg []byte
var recipients []*openpgp.Entity

func init() {
	block, err := armor.Decode(bytes.NewBuffer(key_gpg))
	if err != nil {
		panic(err)
	}
	recip, err := openpgp.ReadEntity(packet.NewReader(block.Body))
	if err != nil {
		panic(err)
	}
	recipients = append(recipients, recip)
}

func Enc(name string, data []byte) ([]byte, error) {
	w := bytes.NewBuffer(nil)
	wc, err := openpgp.Encrypt(w, recipients, nil, &openpgp.FileHints{
		IsBinary: true, FileName: name, ModTime: time.Now(),
	}, nil)
	if err != nil {
		return nil, err
	}
	if _, err = wc.Write(data); err != nil {
		return nil, err
	}
	wc.Close()
	return w.Bytes(), nil
}

func Encer(name string, w io.Writer) (io.WriteCloser, error) {
	wc, err := openpgp.Encrypt(w, recipients, nil, &openpgp.FileHints{
		IsBinary: true, FileName: name, ModTime: time.Now(),
	}, nil)
	return wc, err
}
