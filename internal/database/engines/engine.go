// Package engines
package engines

import (
	"context"
	"database/sql"
)

type DB interface {
	CreateDatabase(ctx context.Context, db *sql.DB) (error)
}
