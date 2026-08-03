package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

// Manager is a connection manager for database connections
type Manager struct {
	mu sync.RWMutex
	connections map[string]*sql.DB
}

// NewManager returns an empty connection manager
func NewManager() *Manager {
	return &Manager{
		connections: make(map[string]DB),
	}
}

func (m *Manager) Get(
	ctx context.Context,
	instance string,
	cfg Config,
) (DB, error) {
	key := makeKey(cfg)

	m.mu.RLock()
	conn, ok := m.connections[key]
	m.mu.RUnlock()
	if ok {
		return conn, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if conn, ok := m.connections[key]; ok {
		return conn, nil
	}
	conn, err := Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	m.connections[key] = conn
	return conn, nil
}


// Close all database connections
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, conn := range m.connections {
		conn.Close()
	}
	m.connections = make(map[string]DB)
}

func makeKey(cfg Config) string {
	return fmt.Sprintf(
		"%s://%s:%d/%s",
		cfg.Engine,
		cfg.Username,
		cfg.Port,
		cfg.Host,
	)
}
