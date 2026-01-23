package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"

	"github.com/anthropics/agentsmesh/backend/internal/infra/dns"
)

// Config holds ACME manager configuration
type Config struct {
	// ACME directory URL
	// Production: https://acme-v02.api.letsencrypt.org/directory
	// Staging: https://acme-staging-v02.api.letsencrypt.org/directory
	DirectoryURL string

	// Email for Let's Encrypt account registration
	Email string

	// Domain for wildcard certificate (e.g., "relay.agentsmesh.cn")
	// Will request certificate for "*.relay.agentsmesh.cn"
	Domain string

	// Storage directory for certificates and account data
	StorageDir string

	// DNS provider for DNS-01 challenge
	DNSProvider dns.Provider

	// Certificate renewal threshold (default: 30 days before expiry)
	RenewalDays int
}

// Manager handles ACME certificate management
type Manager struct {
	cfg    Config
	client *lego.Client
	user   *acmeUser

	// Current certificate
	cert      *Certificate
	certMu    sync.RWMutex

	logger *slog.Logger
}

// Certificate holds the certificate data
type Certificate struct {
	Domain      string    `json:"domain"`
	Certificate []byte    `json:"certificate"` // PEM encoded certificate chain
	PrivateKey  []byte    `json:"private_key"` // PEM encoded private key
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	IssuedAt    time.Time `json:"issued_at"`
}

// acmeUser implements registration.User interface
type acmeUser struct {
	Email        string                 `json:"email"`
	Registration *registration.Resource `json:"registration"`
	Key          crypto.PrivateKey      `json:"-"`
	KeyPEM       []byte                 `json:"key_pem"`
}

func (u *acmeUser) GetEmail() string {
	return u.Email
}

func (u *acmeUser) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u *acmeUser) GetPrivateKey() crypto.PrivateKey {
	return u.Key
}

// NewManager creates a new ACME manager
func NewManager(cfg Config) (*Manager, error) {
	if cfg.DirectoryURL == "" {
		cfg.DirectoryURL = lego.LEDirectoryProduction
	}
	if cfg.RenewalDays == 0 {
		cfg.RenewalDays = 30
	}
	if cfg.StorageDir == "" {
		cfg.StorageDir = "/var/lib/agentsmesh/acme"
	}

	// Ensure storage directory exists
	if err := os.MkdirAll(cfg.StorageDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	m := &Manager{
		cfg:    cfg,
		logger: slog.With("component", "acme_manager"),
	}

	// Load or create user
	if err := m.loadOrCreateUser(); err != nil {
		return nil, fmt.Errorf("failed to load/create ACME user: %w", err)
	}

	// Create ACME client
	legoConfig := lego.NewConfig(m.user)
	legoConfig.CADirURL = cfg.DirectoryURL
	legoConfig.Certificate.KeyType = certcrypto.EC256

	client, err := lego.NewClient(legoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACME client: %w", err)
	}

	// Set DNS-01 challenge provider
	dnsProvider := &dnsProviderAdapter{provider: cfg.DNSProvider, logger: m.logger}
	if err := client.Challenge.SetDNS01Provider(dnsProvider, dns01.AddRecursiveNameservers([]string{"8.8.8.8:53", "1.1.1.1:53"})); err != nil {
		return nil, fmt.Errorf("failed to set DNS provider: %w", err)
	}

	m.client = client

	// Register user if not already registered
	if m.user.Registration == nil {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, fmt.Errorf("failed to register ACME user: %w", err)
		}
		m.user.Registration = reg
		if err := m.saveUser(); err != nil {
			return nil, fmt.Errorf("failed to save user registration: %w", err)
		}
		m.logger.Info("ACME user registered", "email", cfg.Email)
	}

	// Load existing certificate if available
	if err := m.loadCertificate(); err != nil {
		m.logger.Warn("No existing certificate found", "error", err)
	}

	m.logger.Info("ACME manager initialized",
		"directory", cfg.DirectoryURL,
		"domain", cfg.Domain,
		"email", cfg.Email)

	return m, nil
}

// GetCertificate returns the current certificate
func (m *Manager) GetCertificate() *Certificate {
	m.certMu.RLock()
	defer m.certMu.RUnlock()
	return m.cert
}

// GetCertificatePEM returns certificate and key as PEM strings
func (m *Manager) GetCertificatePEM() (cert string, key string, expiry time.Time, err error) {
	m.certMu.RLock()
	defer m.certMu.RUnlock()

	if m.cert == nil {
		return "", "", time.Time{}, fmt.Errorf("no certificate available")
	}

	return string(m.cert.Certificate), string(m.cert.PrivateKey), m.cert.NotAfter, nil
}

// NeedsRenewal checks if the certificate needs renewal
func (m *Manager) NeedsRenewal() bool {
	m.certMu.RLock()
	defer m.certMu.RUnlock()

	if m.cert == nil {
		return true
	}

	renewalTime := m.cert.NotAfter.AddDate(0, 0, -m.cfg.RenewalDays)
	return time.Now().After(renewalTime)
}

// ObtainCertificate obtains a new wildcard certificate
func (m *Manager) ObtainCertificate(ctx context.Context) error {
	wildcardDomain := "*." + m.cfg.Domain

	m.logger.Info("Obtaining certificate", "domain", wildcardDomain)

	request := certificate.ObtainRequest{
		Domains: []string{wildcardDomain},
		Bundle:  true,
	}

	certificates, err := m.client.Certificate.Obtain(request)
	if err != nil {
		return fmt.Errorf("failed to obtain certificate: %w", err)
	}

	// Parse certificate to get validity dates
	block, _ := pem.Decode(certificates.Certificate)
	if block == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	cert := &Certificate{
		Domain:      wildcardDomain,
		Certificate: certificates.Certificate,
		PrivateKey:  certificates.PrivateKey,
		NotBefore:   x509Cert.NotBefore,
		NotAfter:    x509Cert.NotAfter,
		IssuedAt:    time.Now(),
	}

	m.certMu.Lock()
	m.cert = cert
	m.certMu.Unlock()

	// Save certificate to disk
	if err := m.saveCertificate(); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	m.logger.Info("Certificate obtained successfully",
		"domain", wildcardDomain,
		"not_before", cert.NotBefore,
		"not_after", cert.NotAfter)

	return nil
}

// RenewCertificate renews the current certificate
func (m *Manager) RenewCertificate(ctx context.Context) error {
	m.certMu.RLock()
	currentCert := m.cert
	m.certMu.RUnlock()

	if currentCert == nil {
		return m.ObtainCertificate(ctx)
	}

	m.logger.Info("Renewing certificate", "domain", currentCert.Domain)

	// For renewal, we need to obtain a new certificate
	return m.ObtainCertificate(ctx)
}

// StartAutoRenewal starts background certificate renewal
func (m *Manager) StartAutoRenewal(ctx context.Context) {
	go func() {
		// Check immediately on startup
		if m.NeedsRenewal() {
			m.logger.Info("Certificate needs renewal, obtaining new certificate...")
			if err := m.ObtainCertificate(ctx); err != nil {
				m.logger.Error("Failed to obtain certificate on startup", "error", err)
			}
		}

		// Check daily
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if m.NeedsRenewal() {
					m.logger.Info("Certificate needs renewal, renewing...")
					if err := m.RenewCertificate(ctx); err != nil {
						m.logger.Error("Failed to renew certificate", "error", err)
					}
				}
			}
		}
	}()
}

// loadOrCreateUser loads existing user or creates a new one
func (m *Manager) loadOrCreateUser() error {
	userPath := filepath.Join(m.cfg.StorageDir, "user.json")

	data, err := os.ReadFile(userPath)
	if err == nil {
		// Load existing user
		var user acmeUser
		if err := json.Unmarshal(data, &user); err != nil {
			return fmt.Errorf("failed to unmarshal user: %w", err)
		}

		// Decode private key
		block, _ := pem.Decode(user.KeyPEM)
		if block == nil {
			return fmt.Errorf("failed to decode user key PEM")
		}

		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse user key: %w", err)
		}
		user.Key = key

		m.user = &user
		m.logger.Info("Loaded existing ACME user", "email", user.Email)
		return nil
	}

	// Create new user
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate user key: %w", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal user key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	m.user = &acmeUser{
		Email:  m.cfg.Email,
		Key:    privateKey,
		KeyPEM: keyPEM,
	}

	m.logger.Info("Created new ACME user", "email", m.cfg.Email)
	return nil
}

// saveUser saves user data to disk
func (m *Manager) saveUser() error {
	userPath := filepath.Join(m.cfg.StorageDir, "user.json")

	data, err := json.MarshalIndent(m.user, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	if err := os.WriteFile(userPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write user file: %w", err)
	}

	return nil
}

// loadCertificate loads certificate from disk
func (m *Manager) loadCertificate() error {
	certPath := filepath.Join(m.cfg.StorageDir, "certificate.json")

	data, err := os.ReadFile(certPath)
	if err != nil {
		return err
	}

	var cert Certificate
	if err := json.Unmarshal(data, &cert); err != nil {
		return fmt.Errorf("failed to unmarshal certificate: %w", err)
	}

	m.certMu.Lock()
	m.cert = &cert
	m.certMu.Unlock()

	m.logger.Info("Loaded existing certificate",
		"domain", cert.Domain,
		"not_after", cert.NotAfter)

	return nil
}

// saveCertificate saves certificate to disk
func (m *Manager) saveCertificate() error {
	m.certMu.RLock()
	cert := m.cert
	m.certMu.RUnlock()

	if cert == nil {
		return fmt.Errorf("no certificate to save")
	}

	certPath := filepath.Join(m.cfg.StorageDir, "certificate.json")

	data, err := json.MarshalIndent(cert, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal certificate: %w", err)
	}

	if err := os.WriteFile(certPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write certificate file: %w", err)
	}

	// Also save PEM files for debugging/manual use
	certPEMPath := filepath.Join(m.cfg.StorageDir, "cert.pem")
	keyPEMPath := filepath.Join(m.cfg.StorageDir, "key.pem")

	if err := os.WriteFile(certPEMPath, cert.Certificate, 0644); err != nil {
		m.logger.Warn("Failed to write cert.pem", "error", err)
	}
	if err := os.WriteFile(keyPEMPath, cert.PrivateKey, 0600); err != nil {
		m.logger.Warn("Failed to write key.pem", "error", err)
	}

	return nil
}
