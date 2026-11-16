//go:build !android

package main

// platformInit runs at startup in the data folder, before logging starts.
// Nothing to do on the PC; Android's is in platform_android.go.
func platformInit() {}
