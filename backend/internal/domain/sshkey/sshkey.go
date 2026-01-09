package sshkey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHKey represents an SSH key pair stored at the organization level
type SSHKey struct {
	ID             int64  `gorm:"primaryKey" json:"id"`
	OrganizationID int64  `gorm:"not null;index" json:"organization_id"`
	Name           string `gorm:"size:100;not null" json:"name"`

	// PublicKey is displayed to users (OpenSSH format: ssh-rsa AAAA... name)
	PublicKey string `gorm:"type:text;not null" json:"public_key"`

	// PrivateKeyEnc stores the encrypted private key (PEM format)
	// Currently stored as plain text, encryption will be added in P2
	PrivateKeyEnc string `gorm:"column:private_key_encrypted;type:text;not null" json:"-"`

	// Fingerprint is SHA256 fingerprint of the public key for identification
	Fingerprint string `gorm:"size:100;not null" json:"fingerprint"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (SSHKey) TableName() string {
	return "ssh_keys"
}

// KeyPair holds a generated key pair
type KeyPair struct {
	PublicKey   string
	PrivateKey  string
	Fingerprint string
}

// GenerateKeyPair generates a new RSA key pair (4096 bits)
func GenerateKeyPair(comment string) (*KeyPair, error) {
	// Generate RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Encode private key to PEM format
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Generate public key in OpenSSH format
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH public key: %w", err)
	}

	authorizedKey := ssh.MarshalAuthorizedKey(publicKey)
	// Remove trailing newline and add comment
	publicKeyStr := string(authorizedKey[:len(authorizedKey)-1])
	if comment != "" {
		publicKeyStr += " " + comment
	}

	// Calculate fingerprint
	fingerprint := ssh.FingerprintSHA256(publicKey)

	return &KeyPair{
		PublicKey:   publicKeyStr,
		PrivateKey:  string(privateKeyPEM),
		Fingerprint: fingerprint,
	}, nil
}

// CalculateFingerprint calculates the SHA256 fingerprint from a public key string
func CalculateFingerprint(publicKeyStr string) (string, error) {
	// Parse the authorized key format
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKeyStr))
	if err != nil {
		return "", fmt.Errorf("failed to parse public key: %w", err)
	}

	return ssh.FingerprintSHA256(publicKey), nil
}

// ValidatePrivateKey validates that a private key string is valid PEM-encoded RSA/ED25519 key
func ValidatePrivateKey(privateKeyStr string) error {
	block, _ := pem.Decode([]byte(privateKeyStr))
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		_, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		return err
	case "PRIVATE KEY":
		_, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		return err
	case "OPENSSH PRIVATE KEY":
		// For OpenSSH format, we can try to parse it with ssh package
		_, err := ssh.ParseRawPrivateKey([]byte(privateKeyStr))
		return err
	default:
		return fmt.Errorf("unsupported key type: %s", block.Type)
	}
}

// ExtractPublicKeyFromPrivate extracts the public key from a private key
func ExtractPublicKeyFromPrivate(privateKeyStr string) (string, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privateKeyStr))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	publicKey := signer.PublicKey()
	authorizedKey := ssh.MarshalAuthorizedKey(publicKey)

	return string(authorizedKey[:len(authorizedKey)-1]), nil
}

// HashForComparison returns a hash suitable for comparing keys
func HashForComparison(data string) string {
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}
