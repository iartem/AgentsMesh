package config

import (
	"os"
	"testing"
)

// Tests for org slug persistence

func TestConfigSaveAndLoadOrgSlug(t *testing.T) {
	// Create temp home directory
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	cfg := &Config{}

	// Save org slug
	err := cfg.SaveOrgSlug("test-org")
	if err != nil {
		t.Fatalf("SaveOrgSlug error: %v", err)
	}

	if cfg.OrgSlug != "test-org" {
		t.Errorf("OrgSlug after save: got %v, want test-org", cfg.OrgSlug)
	}

	// Clear and reload
	cfg.OrgSlug = ""
	err = cfg.LoadOrgSlug()
	if err != nil {
		t.Fatalf("LoadOrgSlug error: %v", err)
	}

	if cfg.OrgSlug != "test-org" {
		t.Errorf("OrgSlug after load: got %v, want test-org", cfg.OrgSlug)
	}
}

func TestConfigLoadOrgSlugNotExists(t *testing.T) {
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	cfg := &Config{}

	// Should not error when file doesn't exist
	err := cfg.LoadOrgSlug()
	if err != nil {
		t.Errorf("LoadOrgSlug should not error when file missing: %v", err)
	}
}

func TestConfigLoadOrgSlugSkipsIfSet(t *testing.T) {
	cfg := &Config{
		OrgSlug: "existing-org",
	}

	err := cfg.LoadOrgSlug()
	if err != nil {
		t.Fatalf("LoadOrgSlug error: %v", err)
	}

	if cfg.OrgSlug != "existing-org" {
		t.Errorf("OrgSlug should remain: got %v, want existing-org", cfg.OrgSlug)
	}
}
