// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"jim-dot-tennis/internal/config"
)

// checkEnvironment checks the settings that need no database, before
// anything is opened, and words each problem for the person who looks after
// the server (the public URL and run mode are checked later by
// config.ValidateStartup, once the club settings are loaded).
func checkEnvironment() error {
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("PORT is %q, which is not a port number. Set PORT to a number from 1 to 65535 (for example 8080), or remove it to use 8080", p)
		}
	}
	dbPath := getEnv("DB_PATH", "./tennis.db")
	dir := filepath.Dir(dbPath)
	info, err := os.Stat(dir)
	switch {
	case err != nil:
		return fmt.Errorf("the folder for the database (%s, from DB_PATH=%s) does not exist or cannot be read. Create it, or mount the data volume there, or change DB_PATH", dir, dbPath)
	case !info.IsDir():
		return fmt.Errorf("DB_PATH=%s points inside %s, which is not a folder. Change DB_PATH to a file in a folder", dbPath, dir)
	}
	probe, err := os.CreateTemp(dir, ".write-check-*")
	if err != nil {
		return fmt.Errorf("the app cannot write to the database folder %s (%v). Check that the data volume is mounted and that the app's user may write to it", dir, err)
	}
	probe.Close()
	os.Remove(probe.Name())
	return nil
}

// runMode names the mode for the startup summary.
func runMode() string {
	switch {
	case config.Production():
		return "production"
	case config.DevMode():
		return "development"
	}
	return "standard"
}
