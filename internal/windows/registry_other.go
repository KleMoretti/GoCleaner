//go:build !windows

package windows

import "errors"

const (
	RegistryString       = "REG_SZ"
	RegistryExpandString = "REG_EXPAND_SZ"
	RegistryDWord        = "REG_DWORD"
)

var ErrRegistryUnsupported = errors.New("Windows registry is not supported on this platform")

// RegistryValue is a simplified representation of a Windows registry value.
type RegistryValue struct {
	Name string
	Type string
	Data string
}

func ReadHKCUValues(keyPath string) ([]RegistryValue, error) {
	return nil, ErrRegistryUnsupported
}

func DeleteHKCUValue(keyPath, valueName string) error {
	return ErrRegistryUnsupported
}
