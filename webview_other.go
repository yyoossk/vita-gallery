//go:build !windows

package main

func openEmbeddedUI(url string) bool { return false }
