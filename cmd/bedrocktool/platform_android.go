package main

import "github.com/bedrock-tool/bedrocktool/ui/gui/android"

// platformInit runs at startup in the data folder, before logging starts:
// it saves a crash from the last run and catches crashes from this one.
func platformInit() {
	android.Setup()
}
