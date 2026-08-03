// Package database is an interface for database connection management and session handling.
package database

import "context"

type ConnectionManager interface {
    Session(ctx context.Context, server *DatabaseServer) (*Session, error)
    Close() error
}

type Session struct {
    Admin DB
    User  DB
}
