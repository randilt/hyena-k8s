package shamir

import (
	"bytes"
	"testing"
)

func TestSplitCombine3of5AllThresholdSubsets(t *testing.T) {
	secret := []byte("my-super-secret-database-password-12345")
	shares, err := Split(secret, 5, 3)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	subsets := [][]int{
		{0, 1, 2}, {0, 1, 3}, {0, 1, 4}, {0, 2, 3}, {0, 2, 4},
		{0, 3, 4}, {1, 2, 3}, {1, 2, 4}, {1, 3, 4}, {2, 3, 4},
	}

	for _, idx := range subsets {
		pick := [][]byte{shares[idx[0]], shares[idx[1]], shares[idx[2]]}
		got, err := Combine(pick)
		if err != nil {
			t.Fatalf("Combine subset %v: %v", idx, err)
		}
		if !bytes.Equal(got, secret) {
			t.Fatalf("Combine subset %v: secret mismatch", idx)
		}
	}
}

func TestCombineRejectsBelowThresholdSubset(t *testing.T) {
	secret := []byte("db-password")
	shares, err := Split(secret, 5, 3)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	// k-1 shares reconstruct to incorrect bytes (information-theoretic property).
	got, err := Combine(shares[:2])
	if err != nil {
		t.Fatalf("Combine with 2 shares returned error (expected wrong secret): %v", err)
	}
	if bytes.Equal(got, secret) {
		t.Fatal("Combine with k-1 shares must not recover the original secret")
	}
}

func TestCombineRequiresDistinctShares(t *testing.T) {
	secret := []byte("api-key")
	shares, err := Split(secret, 5, 3)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	duplicate := [][]byte{shares[0], shares[0], shares[1]}
	if _, err := Combine(duplicate); err == nil {
		t.Fatal("expected error when duplicate share x-coordinates are supplied")
	}
}

func TestSplitInvalidParameters(t *testing.T) {
	secret := []byte("jwt-token")
	if _, err := Split(secret, 2, 3); err == nil {
		t.Fatal("expected error when threshold exceeds parts")
	}
	if _, err := Split(nil, 5, 3); err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestDemoSecretsRoundTrip(t *testing.T) {
	for _, secret := range demoSecrets {
		shares, err := Split(secret, 5, 3)
		if err != nil {
			t.Fatalf("Split %q: %v", secret, err)
		}
		got, err := Combine(shares[:3])
		if err != nil {
			t.Fatalf("Combine %q: %v", secret, err)
		}
		if !bytes.Equal(got, secret) {
			t.Fatalf("round-trip failed for secret length %d", len(secret))
		}
	}
}
