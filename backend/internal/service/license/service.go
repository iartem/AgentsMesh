package license

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/config"
	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"gorm.io/gorm"
)

// LicenseData represents the decoded license file structure
type LicenseData struct {
	LicenseKey       string         `json:"license_key"`
	OrganizationName string         `json:"organization_name"`
	ContactEmail     string         `json:"contact_email"`
	Plan             string         `json:"plan"`
	Limits           LicenseLimits  `json:"limits"`
	Features         []string       `json:"features,omitempty"`
	IssuedAt         time.Time      `json:"issued_at"`
	ExpiresAt        time.Time      `json:"expires_at"`
	Signature        string         `json:"signature"`
}

// LicenseLimits defines the resource limits for the license
type LicenseLimits struct {
	MaxUsers        int `json:"max_users"`
	MaxRunners      int `json:"max_runners"`
	MaxRepositories int `json:"max_repositories"`
	MaxPodMinutes   int `json:"max_pod_minutes"` // -1 for unlimited
}

// Service handles license verification and management
type Service struct {
	db          *gorm.DB
	cfg         *config.LicenseConfig
	logger      *slog.Logger
	publicKey   *rsa.PublicKey

	// Cache for current license
	mu             sync.RWMutex
	currentLicense *LicenseData
	lastCheck      time.Time
}

// NewService creates a new license service
func NewService(db *gorm.DB, cfg *config.LicenseConfig, logger *slog.Logger) (*Service, error) {
	svc := &Service{
		db:     db,
		cfg:    cfg,
		logger: logger,
	}

	// Load public key for signature verification
	if cfg.PublicKeyPath != "" {
		publicKey, err := loadPublicKey(cfg.PublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load license public key: %w", err)
		}
		svc.publicKey = publicKey
	}

	// Load license file if specified
	if cfg.LicenseFilePath != "" {
		if err := svc.loadLicenseFile(); err != nil {
			logger.Warn("failed to load license file", "error", err)
			// Don't fail startup, allow activation later
		}
	}

	return svc, nil
}

// loadPublicKey loads an RSA public key from a PEM file
func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

// loadLicenseFile loads and verifies the license file
func (s *Service) loadLicenseFile() error {
	data, err := os.ReadFile(s.cfg.LicenseFilePath)
	if err != nil {
		return fmt.Errorf("failed to read license file: %w", err)
	}

	license, err := s.ParseAndVerify(data)
	if err != nil {
		return fmt.Errorf("failed to verify license: %w", err)
	}

	s.mu.Lock()
	s.currentLicense = license
	s.lastCheck = time.Now()
	s.mu.Unlock()

	s.logger.Info("license loaded successfully",
		"license_key", license.LicenseKey,
		"organization", license.OrganizationName,
		"plan", license.Plan,
		"expires_at", license.ExpiresAt,
	)

	return nil
}

// ParseAndVerify parses license data and verifies the signature
func (s *Service) ParseAndVerify(data []byte) (*LicenseData, error) {
	var license LicenseData
	if err := json.Unmarshal(data, &license); err != nil {
		return nil, fmt.Errorf("failed to parse license JSON: %w", err)
	}

	// Verify signature
	if s.publicKey != nil {
		if err := s.verifySignature(&license); err != nil {
			return nil, fmt.Errorf("signature verification failed: %w", err)
		}
	}

	// Check expiration
	if time.Now().After(license.ExpiresAt) {
		return nil, fmt.Errorf("license has expired")
	}

	return &license, nil
}

// verifySignature verifies the license signature using RSA-SHA256
func (s *Service) verifySignature(license *LicenseData) error {
	// Build the data that was signed (everything except the signature)
	dataToSign := struct {
		LicenseKey       string        `json:"license_key"`
		OrganizationName string        `json:"organization_name"`
		ContactEmail     string        `json:"contact_email"`
		Plan             string        `json:"plan"`
		Limits           LicenseLimits `json:"limits"`
		Features         []string      `json:"features,omitempty"`
		IssuedAt         time.Time     `json:"issued_at"`
		ExpiresAt        time.Time     `json:"expires_at"`
	}{
		LicenseKey:       license.LicenseKey,
		OrganizationName: license.OrganizationName,
		ContactEmail:     license.ContactEmail,
		Plan:             license.Plan,
		Limits:           license.Limits,
		Features:         license.Features,
		IssuedAt:         license.IssuedAt,
		ExpiresAt:        license.ExpiresAt,
	}

	jsonData, err := json.Marshal(dataToSign)
	if err != nil {
		return fmt.Errorf("failed to marshal license data: %w", err)
	}

	// Decode signature from base64
	signature, err := base64.StdEncoding.DecodeString(license.Signature)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// Hash the data
	hash := sha256.Sum256(jsonData)

	// Verify signature
	if err := rsa.VerifyPKCS1v15(s.publicKey, crypto.SHA256, hash[:], signature); err != nil {
		return fmt.Errorf("signature mismatch: %w", err)
	}

	return nil
}

// GetCurrentLicense returns the current active license
func (s *Service) GetCurrentLicense() *LicenseData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentLicense
}

// IsLicenseValid checks if the current license is valid
func (s *Service) IsLicenseValid() bool {
	license := s.GetCurrentLicense()
	if license == nil {
		return false
	}
	return time.Now().Before(license.ExpiresAt)
}

// GetLicenseStatus returns the current license status
func (s *Service) GetLicenseStatus() *billing.LicenseStatus {
	license := s.GetCurrentLicense()

	status := &billing.LicenseStatus{
		IsActive: false,
	}

	if license == nil {
		status.Message = "No license installed"
		return status
	}

	if time.Now().After(license.ExpiresAt) {
		status.IsActive = false
		status.Message = "License has expired"
		status.ExpiresAt = &license.ExpiresAt
		return status
	}

	status.IsActive = true
	status.LicenseKey = license.LicenseKey
	status.OrganizationName = license.OrganizationName
	status.Plan = license.Plan
	status.ExpiresAt = &license.ExpiresAt
	status.MaxUsers = license.Limits.MaxUsers
	status.MaxRunners = license.Limits.MaxRunners
	status.MaxRepositories = license.Limits.MaxRepositories
	status.MaxPodMinutes = license.Limits.MaxPodMinutes
	status.Features = license.Features

	daysUntilExpiry := int(time.Until(license.ExpiresAt).Hours() / 24)
	if daysUntilExpiry <= 30 {
		status.Message = fmt.Sprintf("License expires in %d days", daysUntilExpiry)
	} else {
		status.Message = "License is active"
	}

	return status
}

// ActivateLicense activates a new license from the provided data
func (s *Service) ActivateLicense(ctx context.Context, licenseData []byte) error {
	license, err := s.ParseAndVerify(licenseData)
	if err != nil {
		return fmt.Errorf("invalid license: %w", err)
	}

	// Store in database
	activatedAt := time.Now()
	expiresAt := license.ExpiresAt

	// Convert []string features to map[string]interface{}
	features := make(billing.Features)
	for _, f := range license.Features {
		features[f] = true
	}

	dbLicense := &billing.License{
		LicenseKey:       license.LicenseKey,
		OrganizationName: license.OrganizationName,
		ContactEmail:     license.ContactEmail,
		PlanName:         license.Plan,
		MaxUsers:         license.Limits.MaxUsers,
		MaxRunners:       license.Limits.MaxRunners,
		MaxRepositories:  license.Limits.MaxRepositories,
		// Note: MaxPodMinutes maps to MaxConcurrentPods in DB model
		MaxConcurrentPods: license.Limits.MaxPodMinutes,
		Features:          features,
		IssuedAt:          license.IssuedAt,
		ExpiresAt:         &expiresAt,
		Signature:         license.Signature,
		ActivatedAt:       &activatedAt,
		IsActive:          true,
	}

	// Deactivate any existing licenses
	if err := s.db.WithContext(ctx).
		Model(&billing.License{}).
		Where("is_active = ?", true).
		Update("is_active", false).Error; err != nil {
		s.logger.Warn("failed to deactivate existing licenses", "error", err)
	}

	// Create new license record
	if err := s.db.WithContext(ctx).Create(dbLicense).Error; err != nil {
		return fmt.Errorf("failed to save license: %w", err)
	}

	// Update cached license
	s.mu.Lock()
	s.currentLicense = license
	s.lastCheck = time.Now()
	s.mu.Unlock()

	s.logger.Info("license activated",
		"license_key", license.LicenseKey,
		"organization", license.OrganizationName,
		"plan", license.Plan,
		"expires_at", license.ExpiresAt,
	)

	return nil
}

// CheckLimits checks if the current usage is within license limits
func (s *Service) CheckLimits(users, runners, repositories, podMinutes int) error {
	license := s.GetCurrentLicense()
	if license == nil {
		return fmt.Errorf("no active license")
	}

	if !time.Now().Before(license.ExpiresAt) {
		return fmt.Errorf("license has expired")
	}

	limits := license.Limits

	if limits.MaxUsers != -1 && users > limits.MaxUsers {
		return fmt.Errorf("user limit exceeded: %d/%d", users, limits.MaxUsers)
	}

	if limits.MaxRunners != -1 && runners > limits.MaxRunners {
		return fmt.Errorf("runner limit exceeded: %d/%d", runners, limits.MaxRunners)
	}

	if limits.MaxRepositories != -1 && repositories > limits.MaxRepositories {
		return fmt.Errorf("repository limit exceeded: %d/%d", repositories, limits.MaxRepositories)
	}

	if limits.MaxPodMinutes != -1 && podMinutes > limits.MaxPodMinutes {
		return fmt.Errorf("pod minutes exceeded: %d/%d", podMinutes, limits.MaxPodMinutes)
	}

	return nil
}

// HasFeature checks if a specific feature is enabled in the license
func (s *Service) HasFeature(feature string) bool {
	license := s.GetCurrentLicense()
	if license == nil {
		return false
	}

	for _, f := range license.Features {
		if f == feature {
			return true
		}
	}

	return false
}

// RefreshLicense reloads the license from file
func (s *Service) RefreshLicense() error {
	if s.cfg.LicenseFilePath == "" {
		return fmt.Errorf("no license file path configured")
	}
	return s.loadLicenseFile()
}
