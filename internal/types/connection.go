package types

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// SSLMode is a PostgreSQL sslmode value.
type SSLMode string

const (
	SSLModeDisable    SSLMode = "disable"
	SSLModeAllow      SSLMode = "allow"
	SSLModePrefer     SSLMode = "prefer"
	SSLModeRequire    SSLMode = "require"
	SSLModeVerifyCA   SSLMode = "verify-ca"
	SSLModeVerifyFull SSLMode = "verify-full"
)

// Connection stores PostgreSQL connection details.
type Connection struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Host      string     `json:"host"`
	Port      int        `json:"port"`
	Username  string     `json:"username"`
	Password  string     `json:"password,omitempty"` // #nosec G117 -- stored in local user config.
	Database  string     `json:"database,omitempty"`
	SSLMode   SSLMode    `json:"ssl_mode,omitempty"`
	Group     string     `json:"group,omitempty"`
	Color     string     `json:"color,omitempty"`
	ReadOnly  bool       `json:"read_only,omitempty"`
	TLSConfig *TLSConfig `json:"tls_config,omitempty"`
	Created   time.Time  `json:"created_at"`
	Updated   time.Time  `json:"updated_at"`
}

// Address returns host:port.
func (c Connection) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// DSN builds a PostgreSQL connection string for pgx.
func (c Connection) DSN() string {
	user := url.UserPassword(c.Username, c.Password)
	if c.Password == "" {
		user = url.User(c.Username)
	}
	host := c.Host
	if c.Port > 0 {
		host = fmt.Sprintf("%s:%d", c.Host, c.Port)
	}
	db := c.Database
	if db == "" {
		db = "postgres"
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   user,
		Host:   host,
		Path:   "/" + db,
	}
	q := url.Values{}
	ssl := string(c.SSLMode)
	if ssl == "" {
		ssl = string(SSLModePrefer)
	}
	q.Set("sslmode", ssl)
	if c.TLSConfig != nil {
		if c.TLSConfig.CAFile != "" {
			q.Set("sslrootcert", c.TLSConfig.CAFile)
		}
		if c.TLSConfig.CertFile != "" {
			q.Set("sslcert", c.TLSConfig.CertFile)
		}
		if c.TLSConfig.KeyFile != "" {
			q.Set("sslkey", c.TLSConfig.KeyFile)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// DisplayHost returns a compact host:port[/db] string.
func (c Connection) DisplayHost() string {
	parts := []string{fmt.Sprintf("%s:%d", c.Host, c.Port)}
	if c.Database != "" {
		parts = append(parts, c.Database)
	}
	return strings.Join(parts, "/")
}

// TLSConfig stores optional TLS certificate paths.
type TLSConfig struct {
	CertFile           string `json:"cert_file,omitempty"`
	KeyFile            string `json:"key_file,omitempty"`
	CAFile             string `json:"ca_file,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
	ServerName         string `json:"server_name,omitempty"`
}

// BuildTLSConfig creates a *tls.Config from the stored TLS parameters.
func (t *TLSConfig) BuildTLSConfig() (*tls.Config, error) {
	cfg := &tls.Config{
		InsecureSkipVerify: t.InsecureSkipVerify, // #nosec G402 -- user-configured
		ServerName:         t.ServerName,
	}
	if t.CertFile != "" && t.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS key pair: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	if t.CAFile != "" {
		caCert, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

// ConnectionGroup organizes connections.
type ConnectionGroup struct {
	Name        string  `json:"name"`
	Color       string  `json:"color,omitempty"`
	Connections []int64 `json:"connections"`
	Collapsed   bool    `json:"collapsed,omitempty"`
}

// Favorite stores a favorited object.
type Favorite struct {
	ConnectionID int64     `json:"connection_id"`
	Connection   string    `json:"connection"`
	Database     string    `json:"database"`
	Schema       string    `json:"schema"`
	Object       string    `json:"object"`
	Kind         string    `json:"kind,omitempty"`
	Label        string    `json:"label,omitempty"`
	AddedAt      time.Time `json:"added_at"`
}

// RecentObject tracks recently accessed objects.
type RecentObject struct {
	ConnectionID int64     `json:"connection_id"`
	Database     string    `json:"database"`
	Schema       string    `json:"schema"`
	Object       string    `json:"object"`
	Kind         string    `json:"kind,omitempty"`
	AccessedAt   time.Time `json:"accessed_at"`
}

// SavedQuery is a named SQL snippet.
type SavedQuery struct {
	Name      string    `json:"name"`
	SQL       string    `json:"sql"`
	Database  string    `json:"database,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
