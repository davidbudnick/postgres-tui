package types

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConnectionDSN_Branches(t *testing.T) {
	tests := []struct {
		name string
		c    Connection
		want []string
	}{
		{
			name: "password and port and tls paths",
			c: Connection{
				Host: "h", Port: 5433, Username: "u", Password: "p", Database: "db",
				SSLMode: SSLModeRequire,
				TLSConfig: &TLSConfig{
					CAFile:   "/ca.pem",
					CertFile: "/cert.pem",
					KeyFile:  "/key.pem",
				},
			},
			want: []string{"h:5433", "db", "sslmode=require", "sslrootcert", "sslcert", "sslkey"},
		},
		{
			name: "empty password defaults ssl and database",
			c:    Connection{Host: "h", Username: "u"},
			want: []string{"h", "postgres", "sslmode=prefer"},
		},
		{
			name: "zero port no hostport",
			c:    Connection{Host: "onlyhost", Username: "u", Password: ""},
			want: []string{"onlyhost", "postgres"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.c.DSN()
			for _, w := range tt.want {
				if !contains(dsn, w) {
					t.Fatalf("dsn %q missing %q", dsn, w)
				}
			}
		})
	}
}

func TestBuildTLSConfig(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, caPath := writeTestCerts(t, dir)

	t.Run("full", func(t *testing.T) {
		tc := &TLSConfig{
			CertFile:           certPath,
			KeyFile:            keyPath,
			CAFile:             caPath,
			InsecureSkipVerify: true,
			ServerName:         "db.example",
		}
		cfg, err := tc.BuildTLSConfig()
		if err != nil {
			t.Fatal(err)
		}
		if !cfg.InsecureSkipVerify || cfg.ServerName != "db.example" {
			t.Fatalf("%+v", cfg)
		}
		if len(cfg.Certificates) != 1 {
			t.Fatal("expected cert")
		}
		if cfg.RootCAs == nil {
			t.Fatal("expected root cas")
		}
	})

	t.Run("bad key pair", func(t *testing.T) {
		tc := &TLSConfig{CertFile: filepath.Join(dir, "missing.crt"), KeyFile: filepath.Join(dir, "missing.key")}
		if _, err := tc.BuildTLSConfig(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("bad ca file", func(t *testing.T) {
		tc := &TLSConfig{CAFile: filepath.Join(dir, "missing-ca.pem")}
		if _, err := tc.BuildTLSConfig(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid ca pem", func(t *testing.T) {
		bad := filepath.Join(dir, "bad-ca.pem")
		if err := os.WriteFile(bad, []byte("not a cert"), 0o600); err != nil {
			t.Fatal(err)
		}
		tc := &TLSConfig{CAFile: bad}
		if _, err := tc.BuildTLSConfig(); err == nil {
			t.Fatal("expected parse error")
		}
	})

	t.Run("empty", func(t *testing.T) {
		cfg, err := (&TLSConfig{}).BuildTLSConfig()
		if err != nil || cfg == nil {
			t.Fatal(err)
		}
	})
}

func TestLogWriterRingBuffer(t *testing.T) {
	w := NewLogWriter()
	for i := 0; i < MaxLogs+5; i++ {
		if _, err := w.Write([]byte("log line")); err != nil {
			t.Fatal(err)
		}
	}
	if w.Len() != MaxLogs {
		t.Fatalf("len=%d", w.Len())
	}
	logs := w.GetLogs()
	if len(logs) != MaxLogs {
		t.Fatalf("logs=%d", len(logs))
	}
}

func writeTestCerts(t *testing.T, dir string) (certPath, keyPath, caPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	caPath = filepath.Join(dir, "ca.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath, caPath
}
