package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultCityID = "5"

func LoadUserState() (map[string]any, error) {
	path, err := userStatePath()
	if err != nil {
		return nil, err
	}
	raw, err := readUserStateRaw(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultUserState(), nil
		}
		return nil, err
	}
	merged := mergeUserState(DefaultUserState(), raw)
	return merged, nil
}

func SaveUserState(update map[string]any) error {
	if update == nil {
		return errors.New("state is nil")
	}
	path, err := userStatePath()
	if err != nil {
		return err
	}

	rawExisting, err := readUserStateRaw(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	merged := mergeUserState(DefaultUserState(), rawExisting)
	merged = mergeUserState(merged, update)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func DefaultUserState() map[string]any {
	return map[string]any{
		"city_id":     defaultCityID,
		"unit_id":     nil,
		"dep_id":      nil,
		"doctor_id":   nil,
		"member_id":   nil,
		"target_dates": []string{},
		"target_date": defaultTargetDate(),
		"time_slots":  []string{"am", "pm"},
		"proxy_submit_enabled": true,
		"preferred_hours": []string{},
		"schedule_id": "",
		"time_types": []string{},
	}
}

func userStatePath() (string, error) {
	configDir, err := resolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "user_state.json"), nil
}

func readUserStateRaw(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return normalizeUserState(payload), nil
}

func mergeUserState(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range overlay {
		out[key] = value
	}
	return normalizeUserState(out)
}

func normalizeUserState(state map[string]any) map[string]any {
	if state == nil {
		return nil
	}

	if cityValue, ok := state["city_id"]; ok {
		city := strings.TrimSpace(toString(cityValue))
		if city == "" {
			city = defaultCityID
		}
		state["city_id"] = city
	} else {
		state["city_id"] = defaultCityID
	}

	if dateValue, ok := state["target_date"]; ok {
		date := strings.TrimSpace(toString(dateValue))
		if date == "" {
			date = defaultTargetDate()
		}
		state["target_date"] = date
	} else {
		state["target_date"] = defaultTargetDate()
	}

	if datesValue, ok := state["target_dates"]; ok {
		state["target_dates"] = normalizeStringSlice(datesValue)
	} else {
		state["target_dates"] = []string{}
	}

	if slotValue, ok := state["time_slots"]; ok {
		state["time_slots"] = normalizeTimeSlots(slotValue)
	} else {
		state["time_slots"] = []string{"am", "pm"}
	}

	if proxyValue, ok := state["proxy_submit_enabled"]; ok {
		state["proxy_submit_enabled"] = normalizeBool(proxyValue, true)
	} else {
		state["proxy_submit_enabled"] = true
	}

	if hoursValue, ok := state["preferred_hours"]; ok {
		state["preferred_hours"] = normalizeStringSlice(hoursValue)
	} else {
		state["preferred_hours"] = []string{}
	}

	if scheduleValue, ok := state["schedule_id"]; ok {
		state["schedule_id"] = strings.TrimSpace(toString(scheduleValue))
	} else {
		state["schedule_id"] = ""
	}

	if typesValue, ok := state["time_types"]; ok {
		state["time_types"] = normalizeStringSlice(typesValue)
	} else {
		state["time_types"] = []string{}
	}

	return state
}

func normalizeBool(value any, defaultValue bool) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		text := strings.ToLower(strings.TrimSpace(v))
		if text == "" {
			return defaultValue
		}
		if text == "1" || text == "true" || text == "yes" || text == "on" {
			return true
		}
		if text == "0" || text == "false" || text == "no" || text == "off" {
			return false
		}
	case float64:
		return v != 0
	case float32:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	case int32:
		return v != 0
	}
	return defaultValue
}

func normalizeTimeSlots(value any) []string {
	switch v := value.(type) {
	case []string:
		if len(v) > 0 {
			return v
		}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			text := strings.TrimSpace(toString(item))
			if text != "" {
				out = append(out, text)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []string{"am", "pm"}
}

func normalizeStringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return trimStringSlice(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			text := strings.TrimSpace(toString(item))
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return nil
		}
		return []string{text}
	default:
		return nil
	}
}

func defaultTargetDate() string {
	return time.Now().AddDate(0, 0, 7).Format("2006-01-02")
}
