//go:build gui || android

package main

import "github.com/bedrock-tool/bedrocktool/ui/gui"

func init() {
	uis["gui"] = &gui.GUI{}
}
