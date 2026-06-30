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
		return nil, fmt.Errorf("打开 HKCU 注册表键失败: %w", err)
	}
	defer key.Close()

	names, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, fmt.Errorf("读取 HKCU 注册表值名称失败: %w", err)
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
			return nil, fmt.Errorf("读取注册表字符串值 %q 失败: %w", name, err)
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
		return fmt.Errorf("打开 HKCU 注册表键失败: %w", err)
	}
	defer key.Close()
	if err := key.DeleteValue(valueName); err != nil {
		return fmt.Errorf("删除 HKCU 注册表值 %q 失败: %w", valueName, err)
	}
	return nil
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
