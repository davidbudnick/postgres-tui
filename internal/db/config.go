package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

var jsonMarshalIndent = json.MarshalIndent

// Config stores all application configuration.
type Config struct {
	Connections   []types.Connection      `json:"connections"`
	Groups        []types.ConnectionGroup `json:"groups,omitempty"`
	Favorites     []types.Favorite        `json:"favorites,omitempty"`
	RecentObjects []types.RecentObject    `json:"recent_objects,omitempty"`
	SavedQueries  []types.SavedQuery      `json:"saved_queries,omitempty"`
	KeyBindings   types.KeyBindings       `json:"key_bindings"`
	MaxRecent     int                     `json:"max_recent_objects"`
	PageSize      int                     `json:"page_size,omitempty"`
	nextID        int64
	path          string
	mu            sync.RWMutex
}

// NewConfig creates or loads configuration from the given path.
func NewConfig(configPath string) (*Config, error) {
	c := &Config{
		path:          configPath,
		Connections:   []types.Connection{},
		Groups:        []types.ConnectionGroup{},
		Favorites:     []types.Favorite{},
		RecentObjects: []types.RecentObject{},
		SavedQueries:  []types.SavedQuery{},
		KeyBindings:   types.DefaultKeyBindings(),
		MaxRecent:     20,
		PageSize:      100,
		nextID:        1,
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}

	if err := c.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	for _, conn := range c.Connections {
		if conn.ID >= c.nextID {
			c.nextID = conn.ID + 1
		}
	}
	if c.PageSize <= 0 {
		c.PageSize = 100
	}
	if c.KeyBindings.Quit == "" {
		c.KeyBindings = types.DefaultKeyBindings()
	}

	return c, nil
}

func (c *Config) load() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c)
}

func (c *Config) save() error {
	safeConnections := make([]types.Connection, len(c.Connections))
	for i, conn := range c.Connections {
		safeConnections[i] = conn
		safeConnections[i].Password = ""
	}

	safeCfg := &Config{
		Connections:   safeConnections,
		Groups:        c.Groups,
		Favorites:     c.Favorites,
		RecentObjects: c.RecentObjects,
		SavedQueries:  c.SavedQueries,
		KeyBindings:   c.KeyBindings,
		MaxRecent:     c.MaxRecent,
		PageSize:      c.PageSize,
	}

	data, err := jsonMarshalIndent(safeCfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o600)
}

// Close implements ConfigService.
func (c *Config) Close() error { return nil }

// ListConnections returns all saved connections sorted by ID.
func (c *Config) ListConnections() ([]types.Connection, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]types.Connection, len(c.Connections))
	copy(result, c.Connections)
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// AddConnection adds a new connection.
func (c *Config) AddConnection(conn types.Connection) (types.Connection, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	conn.ID = c.nextID
	conn.Created = now
	conn.Updated = now
	if conn.SSLMode == "" {
		conn.SSLMode = types.SSLModePrefer
	}
	if conn.Port == 0 {
		conn.Port = 5432
	}
	c.nextID++
	c.Connections = append(c.Connections, conn)

	if err := c.save(); err != nil {
		c.Connections = c.Connections[:len(c.Connections)-1]
		c.nextID--
		return types.Connection{}, err
	}
	return c.Connections[len(c.Connections)-1], nil
}

// UpdateConnection updates an existing connection.
func (c *Config) UpdateConnection(conn types.Connection) (types.Connection, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, existing := range c.Connections {
		if existing.ID == conn.ID {
			if conn.Group == "" {
				conn.Group = existing.Group
			}
			if conn.Color == "" {
				conn.Color = existing.Color
			}
			if conn.TLSConfig == nil {
				conn.TLSConfig = existing.TLSConfig
			}
			if conn.Password == "" {
				conn.Password = existing.Password
			}
			conn.Created = existing.Created
			conn.Updated = time.Now()
			c.Connections[i] = conn
			if err := c.save(); err != nil {
				c.Connections[i] = existing
				return types.Connection{}, err
			}
			return conn, nil
		}
	}
	return types.Connection{}, os.ErrNotExist
}

// DeleteConnection removes a connection by ID.
func (c *Config) DeleteConnection(id int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, conn := range c.Connections {
		if conn.ID == id {
			c.Connections = append(c.Connections[:i], c.Connections[i+1:]...)
			return c.save()
		}
	}
	return os.ErrNotExist
}

// AddFavorite adds an object to favorites.
func (c *Config) AddFavorite(f types.Favorite) (types.Favorite, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, existing := range c.Favorites {
		if existing.ConnectionID == f.ConnectionID && existing.Database == f.Database &&
			existing.Schema == f.Schema && existing.Object == f.Object {
			return existing, nil
		}
	}
	f.AddedAt = time.Now()
	c.Favorites = append(c.Favorites, f)
	if err := c.save(); err != nil {
		c.Favorites = c.Favorites[:len(c.Favorites)-1]
		return types.Favorite{}, err
	}
	return f, nil
}

// RemoveFavorite removes a favorite.
func (c *Config) RemoveFavorite(connID int64, database, schema, object string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, f := range c.Favorites {
		if f.ConnectionID == connID && f.Database == database && f.Schema == schema && f.Object == object {
			c.Favorites = append(c.Favorites[:i], c.Favorites[i+1:]...)
			return c.save()
		}
	}
	return os.ErrNotExist
}

// ListFavorites returns favorites for a connection.
func (c *Config) ListFavorites(connID int64) []types.Favorite {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []types.Favorite
	for _, f := range c.Favorites {
		if f.ConnectionID == connID || connID == 0 {
			out = append(out, f)
		}
	}
	return out
}

// IsFavorite reports whether an object is favorited.
func (c *Config) IsFavorite(connID int64, database, schema, object string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, f := range c.Favorites {
		if f.ConnectionID == connID && f.Database == database && f.Schema == schema && f.Object == object {
			return true
		}
	}
	return false
}

// AddRecentObject records a recently accessed object.
func (c *Config) AddRecentObject(r types.RecentObject) {
	c.mu.Lock()
	defer c.mu.Unlock()
	filtered := c.RecentObjects[:0]
	for _, existing := range c.RecentObjects {
		if !(existing.ConnectionID == r.ConnectionID && existing.Database == r.Database &&
			existing.Schema == r.Schema && existing.Object == r.Object) {
			filtered = append(filtered, existing)
		}
	}
	r.AccessedAt = time.Now()
	c.RecentObjects = append([]types.RecentObject{r}, filtered...)
	if c.MaxRecent > 0 && len(c.RecentObjects) > c.MaxRecent {
		c.RecentObjects = c.RecentObjects[:c.MaxRecent]
	}
	_ = c.save()
}

// ListRecentObjects returns recent objects for a connection.
func (c *Config) ListRecentObjects(connID int64) []types.RecentObject {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []types.RecentObject
	for _, r := range c.RecentObjects {
		if r.ConnectionID == connID || connID == 0 {
			out = append(out, r)
		}
	}
	return out
}

// ListSavedQueries returns saved queries.
func (c *Config) ListSavedQueries() []types.SavedQuery {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]types.SavedQuery, len(c.SavedQueries))
	copy(out, c.SavedQueries)
	return out
}

// AddSavedQuery adds a saved query.
func (c *Config) AddSavedQuery(q types.SavedQuery) (types.SavedQuery, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	q.CreatedAt = time.Now()
	c.SavedQueries = append(c.SavedQueries, q)
	if err := c.save(); err != nil {
		c.SavedQueries = c.SavedQueries[:len(c.SavedQueries)-1]
		return types.SavedQuery{}, err
	}
	return q, nil
}

// DeleteSavedQuery removes a saved query by name.
func (c *Config) DeleteSavedQuery(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, q := range c.SavedQueries {
		if q.Name == name {
			c.SavedQueries = append(c.SavedQueries[:i], c.SavedQueries[i+1:]...)
			return c.save()
		}
	}
	return os.ErrNotExist
}

// GetKeyBindings returns key bindings.
func (c *Config) GetKeyBindings() types.KeyBindings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.KeyBindings
}

// SetKeyBindings sets key bindings.
func (c *Config) SetKeyBindings(kb types.KeyBindings) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.KeyBindings = kb
	return c.save()
}

// ResetKeyBindings restores defaults.
func (c *Config) ResetKeyBindings() error {
	return c.SetKeyBindings(types.DefaultKeyBindings())
}

// GetPageSize returns the configured page size.
func (c *Config) GetPageSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.PageSize <= 0 {
		return 100
	}
	return c.PageSize
}
