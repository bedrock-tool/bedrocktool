package utils_test

import (
	"fmt"
	"testing"

	"github.com/bedrock-tool/bedrocktool/utils"
)

var invalidJson = []byte(`{"test": 001 /* comment */}frgvejmdorgvm`)

func TestInvalidJsonFix(t *testing.T) {
	var test struct {
		Test int `json:"test"`
	}
	err := utils.ParseJson(invalidJson, &test)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%+#v\n", test)
}
