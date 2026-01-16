package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// LicenseData represents the license file structure
type LicenseData struct {
	LicenseKey       string        `json:"license_key"`
	OrganizationName string        `json:"organization_name"`
	ContactEmail     string        `json:"contact_email"`
	Plan             string        `json:"plan"`
	Limits           LicenseLimits `json:"limits"`
	Features         []string      `json:"features,omitempty"`
	IssuedAt         time.Time     `json:"issued_at"`
	ExpiresAt        time.Time     `json:"expires_at"`
	Signature        string        `json:"signature"`
}

// LicenseLimits defines the resource limits
type LicenseLimits struct {
	MaxUsers        int `json:"max_users"`
	MaxRunners      int `json:"max_runners"`
	MaxRepositories int `json:"max_repositories"`
	MaxPodMinutes   int `json:"max_pod_minutes"`
}

// PlanDefaults defines default limits for each plan type
var PlanDefaults = map[string]LicenseLimits{
	"starter": {
		MaxUsers:        5,
		MaxRunners:      2,
		MaxRepositories: 10,
		MaxPodMinutes:   1000,
	},
	"professional": {
		MaxUsers:        25,
		MaxRunners:      10,
		MaxRepositories: 50,
		MaxPodMinutes:   5000,
	},
	"enterprise": {
		MaxUsers:        -1, // unlimited
		MaxRunners:      -1,
		MaxRepositories: -1,
		MaxPodMinutes:   -1,
	},
}

// PlanFeatures defines features available for each plan
var PlanFeatures = map[string][]string{
	"starter": {
		"basic_agents",
		"git_integration",
	},
	"professional": {
		"basic_agents",
		"git_integration",
		"advanced_agents",
		"team_collaboration",
		"priority_support",
	},
	"enterprise": {
		"basic_agents",
		"git_integration",
		"advanced_agents",
		"team_collaboration",
		"priority_support",
		"sso",
		"audit_logs",
		"custom_integrations",
		"dedicated_support",
	},
}

func main() {
	// Command line flags
	org := flag.String("org", "", "Organization name (required)")
	email := flag.String("email", "", "Contact email (required)")
	plan := flag.String("plan", "professional", "License plan: starter, professional, enterprise")
	maxUsers := flag.Int("max-users", 0, "Max users override (0 = use plan default, -1 = unlimited)")
	maxRunners := flag.Int("max-runners", 0, "Max runners override (0 = use plan default, -1 = unlimited)")
	maxRepos := flag.Int("max-repositories", 0, "Max repositories override (0 = use plan default, -1 = unlimited)")
	maxPodMinutes := flag.Int("max-pod-minutes", 0, "Max pod minutes override (0 = use plan default, -1 = unlimited)")
	features := flag.String("features", "", "Comma-separated feature list (empty = use plan defaults)")
	expires := flag.String("expires", "", "Expiration date YYYY-MM-DD (required)")
	privateKeyPath := flag.String("private-key", "", "Path to RSA private key PEM file (required)")
	output := flag.String("output", "", "Output file path (required)")
	genKeys := flag.Bool("generate-keys", false, "Generate a new RSA key pair instead of creating a license")
	keyOutput := flag.String("key-output", "./", "Directory to save generated keys")

	flag.Parse()

	// Handle key generation mode
	if *genKeys {
		if err := generateKeyPair(*keyOutput); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating keys: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Validate required flags
	if *org == "" || *email == "" || *expires == "" || *privateKeyPath == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "Error: Missing required arguments")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage: license-gen [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Required options:")
		fmt.Fprintln(os.Stderr, "  -org string          Organization name")
		fmt.Fprintln(os.Stderr, "  -email string        Contact email")
		fmt.Fprintln(os.Stderr, "  -expires string      Expiration date (YYYY-MM-DD)")
		fmt.Fprintln(os.Stderr, "  -private-key string  Path to RSA private key PEM file")
		fmt.Fprintln(os.Stderr, "  -output string       Output file path")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Optional options:")
		fmt.Fprintln(os.Stderr, "  -plan string         Plan type: starter, professional, enterprise (default: professional)")
		fmt.Fprintln(os.Stderr, "  -max-users int       Override max users (-1 = unlimited)")
		fmt.Fprintln(os.Stderr, "  -max-runners int     Override max runners (-1 = unlimited)")
		fmt.Fprintln(os.Stderr, "  -max-repositories int Override max repositories (-1 = unlimited)")
		fmt.Fprintln(os.Stderr, "  -max-pod-minutes int Override max pod minutes (-1 = unlimited)")
		fmt.Fprintln(os.Stderr, "  -features string     Comma-separated feature list")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Key Generation:")
		fmt.Fprintln(os.Stderr, "  -generate-keys       Generate a new RSA key pair")
		fmt.Fprintln(os.Stderr, "  -key-output string   Directory to save generated keys (default: ./)")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  # Generate key pair first")
		fmt.Fprintln(os.Stderr, "  license-gen -generate-keys -key-output /path/to/keys/")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  # Create a license")
		fmt.Fprintln(os.Stderr, "  license-gen -org \"Acme Corp\" -email \"admin@acme.com\" -plan enterprise \\")
		fmt.Fprintln(os.Stderr, "    -expires 2025-12-31 -private-key private.pem -output license.json")
		os.Exit(1)
	}

	// Validate plan
	defaults, ok := PlanDefaults[*plan]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: Invalid plan '%s'. Valid plans: starter, professional, enterprise\n", *plan)
		os.Exit(1)
	}

	// Parse expiration date
	expiresAt, err := time.Parse("2006-01-02", *expires)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid expiration date format. Use YYYY-MM-DD\n")
		os.Exit(1)
	}
	// Set to end of day
	expiresAt = expiresAt.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	// Load private key
	privateKey, err := loadPrivateKey(*privateKeyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading private key: %v\n", err)
		os.Exit(1)
	}

	// Build license data
	license := LicenseData{
		LicenseKey:       generateLicenseKey(*plan),
		OrganizationName: *org,
		ContactEmail:     *email,
		Plan:             *plan,
		Limits:           defaults,
		IssuedAt:         time.Now().UTC(),
		ExpiresAt:        expiresAt.UTC(),
	}

	// Apply overrides
	if *maxUsers != 0 {
		license.Limits.MaxUsers = *maxUsers
	}
	if *maxRunners != 0 {
		license.Limits.MaxRunners = *maxRunners
	}
	if *maxRepos != 0 {
		license.Limits.MaxRepositories = *maxRepos
	}
	if *maxPodMinutes != 0 {
		license.Limits.MaxPodMinutes = *maxPodMinutes
	}

	// Set features
	if *features != "" {
		license.Features = strings.Split(*features, ",")
		for i := range license.Features {
			license.Features[i] = strings.TrimSpace(license.Features[i])
		}
	} else {
		license.Features = PlanFeatures[*plan]
	}

	// Sign the license
	signature, err := signLicense(&license, privateKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error signing license: %v\n", err)
		os.Exit(1)
	}
	license.Signature = signature

	// Write to file
	outputData, err := json.MarshalIndent(license, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling license: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*output, outputData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing license file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("License generated successfully!\n")
	fmt.Printf("  License Key: %s\n", license.LicenseKey)
	fmt.Printf("  Organization: %s\n", license.OrganizationName)
	fmt.Printf("  Plan: %s\n", license.Plan)
	fmt.Printf("  Expires: %s\n", license.ExpiresAt.Format("2006-01-02"))
	fmt.Printf("  Output: %s\n", *output)
}

// generateLicenseKey generates a unique license key
func generateLicenseKey(plan string) string {
	// Format: AM-{PLAN}-{YEAR}-{RANDOM}
	planPrefix := strings.ToUpper(plan[:3])
	year := time.Now().Year()

	// Generate random suffix
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomStr := strings.ToUpper(fmt.Sprintf("%x", randomBytes))

	return fmt.Sprintf("AM-%s-%d-%s", planPrefix, year, randomStr)
}

// loadPrivateKey loads an RSA private key from a PEM file
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	// Try PKCS8 first
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA private key")
		}
		return rsaKey, nil
	}

	// Try PKCS1
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return rsaKey, nil
}

// signLicense signs the license data using RSA-SHA256
func signLicense(license *LicenseData, privateKey *rsa.PrivateKey) (string, error) {
	// Build the data to sign (everything except signature)
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
		return "", fmt.Errorf("failed to marshal license data: %w", err)
	}

	// Hash the data
	hash := sha256.Sum256(jsonData)

	// Sign with RSA PKCS1v15
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// generateKeyPair generates a new RSA key pair
func generateKeyPair(outputDir string) error {
	fmt.Println("Generating RSA 4096-bit key pair...")

	// Generate key
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	// Encode private key
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write private key
	privateKeyPath := fmt.Sprintf("%s/license_private.pem", strings.TrimSuffix(outputDir, "/"))
	if err := os.WriteFile(privateKeyPath, privateKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Write public key
	publicKeyPath := fmt.Sprintf("%s/license_public.pem", strings.TrimSuffix(outputDir, "/"))
	if err := os.WriteFile(publicKeyPath, publicKeyPEM, 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	fmt.Printf("Key pair generated successfully!\n")
	fmt.Printf("  Private key: %s (keep this secure, use for signing licenses)\n", privateKeyPath)
	fmt.Printf("  Public key: %s (deploy to OnPremise installations for verification)\n", publicKeyPath)

	return nil
}
