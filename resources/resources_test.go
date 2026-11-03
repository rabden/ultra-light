package resources

import (
	"bytes"
	"os"
	"testing"
)

// The copy of the host is the one the repository builds (go generate).
func TestRaycastHostIsCurrent(t *testing.T) {
	b, err := os.ReadFile("../raycast/dist/host.js")
	if err != nil {
		t.Skip("no raycast/dist/host.js here")
	}
	if !bytes.Equal(b, RaycastHost) {
		t.Fatal("linux/resources/raycast-host.js is out of date: run go generate ./resources")
	}
}
