//go:build windows

package registry

import (
	"os"
	"testing"
)

func TestRealRegistryScanIsExplicitlyOptIn(t *testing.T) {
	if os.Getenv("GOCLEANER_RUN_REAL_REGISTRY_TESTS") != "1" {
		t.Skip("set GOCLEANER_RUN_REAL_REGISTRY_TESTS=1 to run real HKCU registry test")
	}

	result, err := ScanInvalidStartup()
	if err != nil {
		t.Fatalf("ScanInvalidStartup returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ScanInvalidStartup returned nil result")
	}
}
