package mcpapp

import (
	"os"
	"strings"
)

// ParseBoolEnv reads a boolean environment variable; empty means defaultTrue.
func ParseBoolEnv(key string, defaultTrue bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return defaultTrue
	}
	switch v {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
