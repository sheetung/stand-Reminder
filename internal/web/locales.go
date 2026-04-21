package web

import (
	"bytes"
	"encoding/json"
	"sync"
)

var localeCache sync.Map

func LocaleText(locale, key, fallback string) string {
	values, err := loadLocaleStrings(locale)
	if err != nil {
		return fallback
	}
	if value, ok := values[key]; ok && value != "" {
		return value
	}
	return fallback
}

func loadLocaleStrings(locale string) (map[string]string, error) {
	name := "zh.json"
	if locale == "en" || locale == "en-US" {
		name = "en.json"
	}

	if cached, ok := localeCache.Load(name); ok {
		return cached.(map[string]string), nil
	}

	data, err := assets.ReadFile("locales/" + name)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	values := make(map[string]string, len(raw))
	for key, msg := range raw {
		var value string
		if err := json.Unmarshal(msg, &value); err == nil {
			values[key] = value
		}
	}
	localeCache.Store(name, values)
	return values, nil
}
