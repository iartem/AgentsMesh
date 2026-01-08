package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localesFS embed.FS

// Locale represents a loaded locale with its translations
type Locale struct {
	Code         string            // e.g., "en", "zh-CN"
	Name         string            // e.g., "English", "简体中文"
	Translations map[string]string // key -> translated string
}

// I18n manages internationalization
type I18n struct {
	mu             sync.RWMutex
	defaultLocale  string
	currentLocale  string
	locales        map[string]*Locale
	fallbackChain  []string
}

// Config holds i18n configuration
type Config struct {
	DefaultLocale string   // Default locale code
	FallbackChain []string // Fallback locales if key not found
}

// DefaultConfig returns a default i18n configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultLocale: "en",
		FallbackChain: []string{"en"},
	}
}

// New creates a new I18n instance
func New(cfg *Config) (*I18n, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	i := &I18n{
		defaultLocale: cfg.DefaultLocale,
		currentLocale: cfg.DefaultLocale,
		locales:       make(map[string]*Locale),
		fallbackChain: cfg.FallbackChain,
	}

	// Load embedded locales
	if err := i.loadEmbeddedLocales(); err != nil {
		return nil, fmt.Errorf("failed to load embedded locales: %w", err)
	}

	return i, nil
}

// loadEmbeddedLocales loads locale files from embedded filesystem
func (i *I18n) loadEmbeddedLocales() error {
	return fs.WalkDir(localesFS, "locales", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := localesFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read locale file %s: %w", path, err)
		}

		// Extract locale code from filename (e.g., "en.json" -> "en")
		code := strings.TrimSuffix(filepath.Base(path), ".json")

		return i.LoadLocaleFromJSON(code, data)
	})
}

// LoadLocaleFromJSON loads a locale from JSON data
func (i *I18n) LoadLocaleFromJSON(code string, data []byte) error {
	var translations map[string]interface{}
	if err := json.Unmarshal(data, &translations); err != nil {
		return fmt.Errorf("failed to parse locale JSON: %w", err)
	}

	locale := &Locale{
		Code:         code,
		Translations: make(map[string]string),
	}

	// Flatten nested translations
	flattenTranslations("", translations, locale.Translations)

	// Extract locale name if present
	if name, ok := locale.Translations["_name"]; ok {
		locale.Name = name
		delete(locale.Translations, "_name")
	} else {
		locale.Name = code
	}

	i.mu.Lock()
	i.locales[code] = locale
	i.mu.Unlock()

	return nil
}

// flattenTranslations flattens nested JSON structure into dot-notation keys
func flattenTranslations(prefix string, data map[string]interface{}, result map[string]string) {
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case string:
			result[fullKey] = v
		case map[string]interface{}:
			flattenTranslations(fullKey, v, result)
		}
	}
}

// SetLocale sets the current locale
func (i *I18n) SetLocale(code string) error {
	i.mu.RLock()
	_, exists := i.locales[code]
	i.mu.RUnlock()

	if !exists {
		return fmt.Errorf("locale %s not found", code)
	}

	i.mu.Lock()
	i.currentLocale = code
	i.mu.Unlock()

	return nil
}

// GetLocale returns the current locale code
func (i *I18n) GetLocale() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.currentLocale
}

// T translates a key using the current locale
func (i *I18n) T(key string, args ...interface{}) string {
	return i.TWithLocale(i.GetLocale(), key, args...)
}

// TWithLocale translates a key using a specific locale
func (i *I18n) TWithLocale(localeCode, key string, args ...interface{}) string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// Try the requested locale
	if locale, ok := i.locales[localeCode]; ok {
		if translation, ok := locale.Translations[key]; ok {
			return formatTranslation(translation, args...)
		}
	}

	// Try fallback chain
	for _, fallback := range i.fallbackChain {
		if fallback == localeCode {
			continue
		}
		if locale, ok := i.locales[fallback]; ok {
			if translation, ok := locale.Translations[key]; ok {
				return formatTranslation(translation, args...)
			}
		}
	}

	// Return key if no translation found
	return key
}

// formatTranslation formats a translation string with arguments
func formatTranslation(template string, args ...interface{}) string {
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

// GetAvailableLocales returns all available locale codes
func (i *I18n) GetAvailableLocales() []LocaleInfo {
	i.mu.RLock()
	defer i.mu.RUnlock()

	locales := make([]LocaleInfo, 0, len(i.locales))
	for code, locale := range i.locales {
		locales = append(locales, LocaleInfo{
			Code: code,
			Name: locale.Name,
		})
	}
	return locales
}

// LocaleInfo contains basic locale information
type LocaleInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Context key for storing locale in context
type localeContextKey struct{}

// WithLocale returns a new context with the specified locale
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeContextKey{}, locale)
}

// GetLocaleFromContext returns the locale from context
func GetLocaleFromContext(ctx context.Context) string {
	if locale, ok := ctx.Value(localeContextKey{}).(string); ok {
		return locale
	}
	return ""
}

// TFromContext translates using locale from context
func (i *I18n) TFromContext(ctx context.Context, key string, args ...interface{}) string {
	locale := GetLocaleFromContext(ctx)
	if locale == "" {
		locale = i.GetLocale()
	}
	return i.TWithLocale(locale, key, args...)
}

// Global instance for convenience
var global *I18n
var globalOnce sync.Once

// Init initializes the global I18n instance
func Init(cfg *Config) error {
	var err error
	globalOnce.Do(func() {
		global, err = New(cfg)
	})
	return err
}

// T translates using the global instance
func T(key string, args ...interface{}) string {
	if global == nil {
		return key
	}
	return global.T(key, args...)
}

// TWithLocale translates using the global instance with a specific locale
func TWithLocale(locale, key string, args ...interface{}) string {
	if global == nil {
		return key
	}
	return global.TWithLocale(locale, key, args...)
}

// TFromContext translates using the global instance with context locale
func TFromContext(ctx context.Context, key string, args ...interface{}) string {
	if global == nil {
		return key
	}
	return global.TFromContext(ctx, key, args...)
}

// SetGlobalLocale sets the global locale
func SetGlobalLocale(code string) error {
	if global == nil {
		return fmt.Errorf("i18n not initialized")
	}
	return global.SetLocale(code)
}

// GetAvailableLocales returns all available locales from global instance
func GetAvailableLocales() []LocaleInfo {
	if global == nil {
		return nil
	}
	return global.GetAvailableLocales()
}
