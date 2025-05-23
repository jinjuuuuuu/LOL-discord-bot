package notification

import (
	"encoding/json"
	"os"
	"sync"
)

const settingsFile = "data/patch_notification_users.json"

var mu sync.Mutex

var userSettings map[string]bool

func init() {
	userSettings = make(map[string]bool)
	loadSettings()
}

func loadSettings() {
	file, err := os.ReadFile(settingsFile)
	if err == nil {
		json.Unmarshal(file, &userSettings)
	}
}

func saveSettings() {
	mu.Lock()
	defer mu.Unlock()

	data, err := json.MarshalIndent(userSettings, "", "  ")
	if err == nil {
		os.WriteFile(settingsFile, data, 0644)
	}
}

func EnablePatchNotification(userID string) {
	userSettings[userID] = true
	saveSettings()
}

func DisablePatchNotification(userID string) {
	delete(userSettings, userID)
	saveSettings()
}

func IsPatchNotificationEnabled(userID string) bool {
	return userSettings[userID]
}

func GetAllNotifiedUsers() []string {
	users := []string{}
	for userID, enabled := range userSettings {
		if enabled {
			users = append(users, userID)
		}
	}
	return users
}
