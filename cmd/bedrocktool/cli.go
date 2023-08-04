package main

import "github.com/bedrock-tool/bedrocktool/ui/cli"

func init() {
	uis["cli"] = &cli.CLI{}
}
