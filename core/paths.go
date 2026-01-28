package core

import (
	"errors"
	"os"
	"path/filepath"
)

const configDirEnv = "QUICKDOCTOR_CONFIG_DIR"

func ConfigDir() (string, error) {
	return resolveConfigDir()
}

func LogsDir() (string, error) {
	return resolveLogsDir()
}

// GrabProfilesPath 返回抢号配置档案文件路径
func GrabProfilesPath() (string, error) {
	configDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "grab_profiles.json"), nil
}

func resolveConfigDir() (string, error) {
	if dir := os.Getenv(configDirEnv); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		return dir, nil
	}

	candidates := make([]string, 0, 6)
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "config"),
			filepath.Join(cwd, "..", "config"),
			filepath.Join(cwd, "..", "..", "config"),
		)
	}
	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(base, "config"),
			filepath.Join(base, "..", "config"),
			filepath.Join(base, "..", "..", "config"),
		)
	}

	for _, dir := range candidates {
		if fileExists(filepath.Join(dir, "cities.json")) {
			return dir, nil
		}
	}

	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err == nil {
			return dir, nil
		}
	}

	return "", errors.New("unable to resolve config directory")
}

func resolveLogsDir() (string, error) {
	configDir, err := resolveConfigDir()
	if err != nil {
		return "", err
	}
	root := filepath.Dir(configDir)
	logsDir := filepath.Join(root, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return "", err
	}
	return logsDir, nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
