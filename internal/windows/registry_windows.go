//go:build windows

package windows

import (
	"fmt"

	winregistry "golang.org/x/sys/windows/registry"
)

const (
	RegistryString       = "REG_SZ"
	RegistryExpandString = "REG_EXPAND_SZ"
	RegistryDWord        = "REG_DWORD"
)

// RegistryValue is a simplified representation of a Windows registry value.
type RegistryValue struct {
	Name string
	Type string
	Data string
}

// ReadHKCUValues reads string-like values from a HKCU registry key.
func ReadHKCUValues(keyPath string) ([]RegistryValue, error) {
	key, err := winregistry.OpenKey(winregistry.CURRENT_USER, keyPath, winregistry.READ)
	if err != nil {
		return nil, err
	}
	defer key.Close()

	names, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, err
	}

	values := make([]RegistryValue, 0, len(names))
	for _, name := range names {
		data, valueType, err := key.GetStringValue(name)
		if err == nil {
			values = append(values, RegistryValue{
				Name: name,
				Type: registryTypeName(valueType),
				Data: data,
			})
			continue
		}
		if valueType == winregistry.SZ || valueType == winregistry.EXPAND_SZ {
			return nil, fmt.Errorf("read registry string value %q: %w", name, err)
		}

		// Non-string startup values are ignored by the scanner.
		values = append(values, RegistryValue{
			Name: name,
			Type: registryTypeName(valueType),
		})
	}

	return values, nil
}

// DeleteHKCUValue deletes one value from a HKCU registry key.
func DeleteHKCUValue(keyPath, valueName string) error {
	key, err := winregistry.OpenKey(winregistry.CURRENT_USER, keyPath, winregistry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.DeleteValue(valueName)
}

func registryTypeName(valueType uint32) string {
	switch valueType {
	case winregistry.SZ:
		return RegistryString
	case winregistry.EXPAND_SZ:
		return RegistryExpandString
	case winregistry.DWORD:
		return RegistryDWord
	default:
		return fmt.Sprintf("REG_%d", valueType)
	}
}
