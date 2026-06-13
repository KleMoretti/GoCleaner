package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gocleaner/internal/model"
	"gocleaner/internal/windows"
)

// PluginTarget identifies a browser User Data directory to scan for extensions.
type PluginTarget struct {
	Browser      string
	UserDataPath string
}

type pluginManifest struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Description   string `json:"description"`
	DefaultLocale string `json:"default_locale"`
}

type localeMessage struct {
	Message string `json:"message"`
}

var chromeMessageRefRe = regexp.MustCompile(`^__MSG_([A-Za-z0-9_]+)__$`)

// DefaultPluginTargets returns the supported Chrome and Edge extension roots.
func DefaultPluginTargets() []PluginTarget {
	return []PluginTarget{
		{
			Browser:      "Chrome",
			UserDataPath: windows.ExpandPath(`%LOCALAPPDATA%\Google\Chrome\User Data`),
		},
		{
			Browser:      "Edge",
			UserDataPath: windows.ExpandPath(`%LOCALAPPDATA%\Microsoft\Edge\User Data`),
		},
	}
}

// ScanBrowserPlugins reads Chrome/Edge extension manifests without modifying files.
func ScanBrowserPlugins(targets []PluginTarget) ([]model.ScanItem, []model.ScanError) {
	var items []model.ScanItem
	var errs []model.ScanError

	for _, target := range targets {
		if strings.TrimSpace(target.Browser) == "" || strings.TrimSpace(target.UserDataPath) == "" {
			continue
		}
		profiles, readErr := os.ReadDir(target.UserDataPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			errs = append(errs, model.ScanError{
				Path:   target.UserDataPath,
				Reason: fmt.Sprintf("插件用户数据目录不可访问: %v", readErr),
			})
			continue
		}

		for _, profile := range profiles {
			if !profile.IsDir() || profile.Type()&os.ModeSymlink != 0 {
				continue
			}
			profileName := profile.Name()
			extensionsPath := filepath.Join(target.UserDataPath, profileName, "Extensions")
			extensions, extErr := os.ReadDir(extensionsPath)
			if extErr != nil {
				if !os.IsNotExist(extErr) {
					errs = append(errs, model.ScanError{
						Path:   extensionsPath,
						Reason: fmt.Sprintf("插件目录不可访问: %v", extErr),
					})
				}
				continue
			}

			for _, extension := range extensions {
				if !extension.IsDir() || extension.Type()&os.ModeSymlink != 0 {
					continue
				}
				item, scanErr := scanExtensionDir(target, profileName, filepath.Join(extensionsPath, extension.Name()))
				if scanErr != nil {
					errs = append(errs, *scanErr)
					continue
				}
				if item != nil {
					items = append(items, *item)
				}
			}
		}
	}

	return items, errs
}

func scanExtensionDir(target PluginTarget, profileName, extensionRoot string) (*model.ScanItem, *model.ScanError) {
	info, statErr := os.Lstat(extensionRoot)
	if statErr != nil {
		return nil, &model.ScanError{Path: extensionRoot, Reason: fmt.Sprintf("插件目录不可访问: %v", statErr)}
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, nil
	}

	versionDir, versionInfo, versionErr := latestManifestVersionDir(extensionRoot)
	if versionErr != nil {
		return nil, &model.ScanError{Path: extensionRoot, Reason: versionErr.Error()}
	}

	manifestPath := filepath.Join(versionDir, "manifest.json")
	raw, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		return nil, &model.ScanError{Path: manifestPath, Reason: fmt.Sprintf("读取插件 manifest 失败: %v", readErr)}
	}

	var manifest pluginManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, &model.ScanError{Path: manifestPath, Reason: fmt.Sprintf("解析插件 manifest 失败: %v", err)}
	}

	extensionID := filepath.Base(extensionRoot)
	name := resolveManifestText(manifest.Name, manifest.DefaultLocale, versionDir, extensionRoot)
	if strings.TrimSpace(name) == "" {
		name = extensionID
	}
	description := resolveManifestText(manifest.Description, manifest.DefaultLocale, versionDir, extensionRoot)
	size := directorySize(extensionRoot)

	return &model.ScanItem{
		ID:           generateID(model.CategoryPlugin, extensionRoot),
		Path:         extensionRoot,
		Name:         name,
		Type:         model.TypePlugin,
		Category:     model.CategoryPlugin,
		Size:         size,
		Risk:         model.RiskMedium,
		Source:       target.Browser + " 插件",
		LastModified: versionInfo.ModTime().Unix(),
		Selected:     false,
		Plugin: &model.PluginInfo{
			Browser:      target.Browser,
			Profile:      profileName,
			ExtensionID:  extensionID,
			Version:      manifest.Version,
			Description:  description,
			ManifestPath: manifestPath,
		},
	}, nil
}

func latestManifestVersionDir(extensionRoot string) (string, os.FileInfo, error) {
	entries, err := os.ReadDir(extensionRoot)
	if err != nil {
		return "", nil, fmt.Errorf("读取插件版本目录失败: %v", err)
	}

	type candidate struct {
		path string
		info os.FileInfo
	}
	var candidates []candidate
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		versionPath := filepath.Join(extensionRoot, entry.Name())
		manifestPath := filepath.Join(versionPath, "manifest.json")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		candidates = append(candidates, candidate{path: versionPath, info: info})
	}
	if len(candidates) == 0 {
		return "", nil, fmt.Errorf("未找到插件 manifest.json")
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return compareVersion(filepath.Base(candidates[i].path), filepath.Base(candidates[j].path)) > 0
	})

	return candidates[0].path, candidates[0].info, nil
}

func compareVersion(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	maxParts := len(aParts)
	if len(bParts) > maxParts {
		maxParts = len(bParts)
	}
	for i := 0; i < maxParts; i++ {
		av := versionPart(aParts, i)
		bv := versionPart(bParts, i)
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return strings.Compare(a, b)
}

func versionPart(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}
	value, err := strconv.Atoi(parts[index])
	if err != nil {
		return 0
	}
	return value
}

func resolveManifestText(value, locale, versionDir, extensionRoot string) string {
	value = strings.TrimSpace(value)
	matches := chromeMessageRefRe.FindStringSubmatch(value)
	if len(matches) != 2 || strings.TrimSpace(locale) == "" {
		return value
	}

	key := matches[1]
	for _, root := range []string{versionDir, extensionRoot} {
		messages, err := readLocaleMessages(filepath.Join(root, "_locales", locale, "messages.json"))
		if err != nil {
			continue
		}
		if msg, ok := messages[key]; ok && strings.TrimSpace(msg.Message) != "" {
			return msg.Message
		}
	}
	return value
}

func readLocaleMessages(path string) (map[string]localeMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var messages map[string]localeMessage
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func directorySize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}
