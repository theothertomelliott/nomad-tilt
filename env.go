package main

import "os"

func getEnvDefault(key, defaultValue string) string {
	out := os.Getenv(key)
	if out != "" {
		return out
	}
	return defaultValue
}
