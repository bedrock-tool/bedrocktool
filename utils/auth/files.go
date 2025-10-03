package auth

import (
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/bedrock-tool/bedrocktool/utils"
)

var Ver1token func(f io.ReadSeeker, o any) error
var Tokene = func(w io.Writer, o any) error {
	return json.NewEncoder(w).Encode(o)
}

func readAuth[T any](name string) (*T, error) {
	f, err := os.Open(utils.PathData(name))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var b = make([]byte, 1)
	_, err = f.ReadAt(b, 0)
	if err != nil {
		return nil, err
	}

	switch b[0] {
	case '{':
		var o T
		e := json.NewDecoder(f)
		err = e.Decode(&o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case '1':
		if Ver1token != nil {
			var o T
			err = Ver1token(f, &o)
			if err != nil {
				return nil, err
			}
			return &o, nil
		}
	}

	return nil, errors.ErrUnsupported
}

func writeAuth(name string, o any) error {
	f, err := os.Create(utils.PathData(name))
	if err != nil {
		return err
	}
	defer f.Close()
	return Tokene(f, o)
}

func tokenFileName(name string) string {
	if name == "" {
		return "token.json"
	}
	return "token-" + name + ".json"
}

func chainFileName(name string) string {
	if name == "" {
		return "chain.bin"
	}
	return "chain-" + name + ".bin"
}
