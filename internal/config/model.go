package config

import (
	"os"
	"path/filepath"
	"strings"
)

var modelFiles = map[string]string{
	"tiny":           "ggml-tiny.bin",
	"base":           "ggml-base.bin",
	"small":          "ggml-small.bin",
	"medium":         "ggml-medium.bin",
	"large-v3":       "ggml-large-v3.bin",
	"large-v3-turbo": "ggml-large-v3-turbo.bin",
}

// userHomeDir is swapped in tests.
var userHomeDir = os.UserHomeDir

// SetUserHomeDirForTest overrides home lookup (tests only).
func SetUserHomeDirForTest(fn func() (string, error)) {
	if fn == nil {
		userHomeDir = os.UserHomeDir
		return
	}
	userHomeDir = fn
}

// ResetUserHomeDirForTest restores os.UserHomeDir.
func ResetUserHomeDirForTest() {
	userHomeDir = os.UserHomeDir
}

// STTLanguage returns the Whisper language code, falling back to assistant.language when stt.language is auto.
func (c Config) STTLanguage() string {
	lang := strings.TrimSpace(c.STT.Language)
	if lang == "" || strings.EqualFold(lang, "auto") {
		if a := strings.TrimSpace(c.Assistant.Language); a != "" && !strings.EqualFold(a, "auto") {
			return a
		}
		return "auto"
	}
	return lang
}

// ResolveSTTModelPath picks a ggml model file from stt.model, stt.model_path, or the default small model.
// stt.model takes precedence over stt.model_path. Shorthand names are searched under the app models dir
// and ~/github/transcription/models/.
func (c Config) ResolveSTTModelPath() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	model := strings.TrimSpace(c.STT.Model)
	modelPath := strings.TrimSpace(c.STT.ModelPath)

	if model != "" {
		return resolveModelCandidate(model, home), nil
	}
	if modelPath != "" {
		if fileExists(modelPath) {
			return modelPath, nil
		}
		if found := findInModelDirs(filepath.Base(modelPath), home); found != "" {
			return found, nil
		}
		return modelPath, nil
	}
	return defaultModelPath(home)
}

func resolveModelCandidate(model, home string) string {
	if strings.Contains(model, "/") || strings.HasSuffix(model, ".bin") {
		if fileExists(model) {
			return model
		}
		if found := findInModelDirs(filepath.Base(model), home); found != "" {
			return found
		}
		return model
	}

	name, ok := modelFiles[model]
	if !ok {
		name = "ggml-" + model + ".bin"
	}
	if found := findInModelDirs(name, home); found != "" {
		return found
	}
	return filepath.Join(defaultModelsDir(home), name)
}

func findInModelDirs(name, home string) string {
	for _, dir := range modelSearchDirs(home) {
		p := filepath.Join(dir, name)
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func modelSearchDirs(home string) []string {
	return []string{
		defaultModelsDir(home),
		filepath.Join(home, "github", "transcription", "models"),
	}
}

func defaultModelsDir(home string) string {
	return filepath.Join(home, ".local", "share", "meeting-sidecar", "models")
}

func defaultModelPath(home string) (string, error) {
	return filepath.Join(defaultModelsDir(home), "ggml-small.bin"), nil
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Size() > 0
}
