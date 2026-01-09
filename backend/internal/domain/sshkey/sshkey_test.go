package sshkey

import (
	"strings"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	keyPair, err := GenerateKeyPair("test-key")
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Check public key format
	if !strings.HasPrefix(keyPair.PublicKey, "ssh-rsa ") {
		t.Errorf("expected public key to start with 'ssh-rsa ', got: %s", keyPair.PublicKey[:20])
	}

	// Check that comment is appended
	if !strings.HasSuffix(keyPair.PublicKey, " test-key") {
		t.Errorf("expected public key to end with ' test-key', got: %s", keyPair.PublicKey[len(keyPair.PublicKey)-20:])
	}

	// Check private key format (PEM)
	if !strings.HasPrefix(keyPair.PrivateKey, "-----BEGIN RSA PRIVATE KEY-----") {
		t.Error("expected private key to be in PEM format")
	}

	// Check fingerprint format
	if !strings.HasPrefix(keyPair.Fingerprint, "SHA256:") {
		t.Errorf("expected fingerprint to start with 'SHA256:', got: %s", keyPair.Fingerprint)
	}
}

func TestGenerateKeyPairEmptyComment(t *testing.T) {
	keyPair, err := GenerateKeyPair("")
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Public key should not have a trailing space when comment is empty
	if strings.HasSuffix(keyPair.PublicKey, " ") {
		t.Error("expected public key not to have trailing space when comment is empty")
	}
}

func TestCalculateFingerprint(t *testing.T) {
	// Generate a key pair first
	keyPair, err := GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Calculate fingerprint from public key
	fingerprint, err := CalculateFingerprint(keyPair.PublicKey)
	if err != nil {
		t.Fatalf("failed to calculate fingerprint: %v", err)
	}

	// Should match the original fingerprint
	if fingerprint != keyPair.Fingerprint {
		t.Errorf("fingerprint mismatch: got %s, expected %s", fingerprint, keyPair.Fingerprint)
	}
}

func TestCalculateFingerprintInvalidKey(t *testing.T) {
	_, err := CalculateFingerprint("invalid-key")
	if err == nil {
		t.Error("expected error for invalid public key")
	}
}

func TestValidatePrivateKeyRSA(t *testing.T) {
	keyPair, err := GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	err = ValidatePrivateKey(keyPair.PrivateKey)
	if err != nil {
		t.Errorf("expected valid RSA private key: %v", err)
	}
}

func TestValidatePrivateKeyInvalid(t *testing.T) {
	testCases := []struct {
		name string
		key  string
	}{
		{"empty string", ""},
		{"random text", "not a key at all"},
		{"invalid PEM", "-----BEGIN FAKE KEY-----\ninvalid\n-----END FAKE KEY-----"},
		{"unsupported key type", "-----BEGIN EC PRIVATE KEY-----\nMHQCAQEEIABCDEF\n-----END EC PRIVATE KEY-----"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePrivateKey(tc.key)
			if err == nil {
				t.Errorf("expected error for invalid key: %s", tc.name)
			}
		})
	}
}

func TestValidatePrivateKeyPKCS8(t *testing.T) {
	// Generate a key pair first to get a valid PKCS1 key
	keyPair, err := GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// PKCS1 format validation should work
	err = ValidatePrivateKey(keyPair.PrivateKey)
	if err != nil {
		t.Errorf("expected valid PKCS1 key, got error: %v", err)
	}
}

func TestValidatePrivateKeyInvalidRSA(t *testing.T) {
	// Invalid RSA key (correct header but invalid content)
	invalidRSAKey := `-----BEGIN RSA PRIVATE KEY-----
MIIBogIBAAJBAKx1c+I
-----END RSA PRIVATE KEY-----`

	err := ValidatePrivateKey(invalidRSAKey)
	if err == nil {
		t.Error("expected error for invalid RSA key")
	}
}

func TestValidatePrivateKeyInvalidPKCS8(t *testing.T) {
	// Invalid PKCS8 key (correct header but invalid content)
	invalidPKCS8Key := `-----BEGIN PRIVATE KEY-----
MIIBogIBAAJBAKx1c+I
-----END PRIVATE KEY-----`

	err := ValidatePrivateKey(invalidPKCS8Key)
	if err == nil {
		t.Error("expected error for invalid PKCS8 key")
	}
}

func TestValidatePrivateKeyInvalidOpenSSH(t *testing.T) {
	// Invalid OpenSSH key (correct header but invalid content)
	invalidOpenSSHKey := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5v
-----END OPENSSH PRIVATE KEY-----`

	err := ValidatePrivateKey(invalidOpenSSHKey)
	if err == nil {
		t.Error("expected error for invalid OpenSSH key")
	}
}

func TestGenerateKeyPairError(t *testing.T) {
	// Test that multiple key generations produce different keys
	keyPair1, err := GenerateKeyPair("test1")
	if err != nil {
		t.Fatalf("failed to generate first key pair: %v", err)
	}

	keyPair2, err := GenerateKeyPair("test2")
	if err != nil {
		t.Fatalf("failed to generate second key pair: %v", err)
	}

	// Keys should be different
	if keyPair1.PublicKey == keyPair2.PublicKey {
		t.Error("expected different public keys for different generations")
	}

	if keyPair1.PrivateKey == keyPair2.PrivateKey {
		t.Error("expected different private keys for different generations")
	}

	if keyPair1.Fingerprint == keyPair2.Fingerprint {
		t.Error("expected different fingerprints for different generations")
	}
}

func TestExtractPublicKeyFromPrivate(t *testing.T) {
	// Generate a key pair
	keyPair, err := GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Extract public key from private key
	publicKey, err := ExtractPublicKeyFromPrivate(keyPair.PrivateKey)
	if err != nil {
		t.Fatalf("failed to extract public key: %v", err)
	}

	// The extracted public key should match the original (minus the comment)
	originalWithoutComment := strings.Split(keyPair.PublicKey, " test")[0]
	if publicKey != originalWithoutComment {
		t.Errorf("extracted public key doesn't match: got %s, expected %s", publicKey[:50], originalWithoutComment[:50])
	}
}

func TestExtractPublicKeyFromPrivateInvalid(t *testing.T) {
	_, err := ExtractPublicKeyFromPrivate("invalid-key")
	if err == nil {
		t.Error("expected error for invalid private key")
	}
}

func TestHashForComparison(t *testing.T) {
	hash1 := HashForComparison("test-data")
	hash2 := HashForComparison("test-data")
	hash3 := HashForComparison("different-data")

	if hash1 != hash2 {
		t.Error("same input should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("different input should produce different hash")
	}

	// Check hash length (SHA256 produces 64 hex characters)
	if len(hash1) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash1))
	}
}

func TestSSHKeyTableName(t *testing.T) {
	key := SSHKey{}
	if key.TableName() != "ssh_keys" {
		t.Errorf("expected table name 'ssh_keys', got '%s'", key.TableName())
	}
}
