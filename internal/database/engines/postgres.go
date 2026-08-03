package engines

import (
	"context"
	"database/sql"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Postgres struct {
	// Database describes the name of the database to create
	Database string
	// Template describes the name of the template database to use when creating a new database
	Template string
}

var _ DB = (*Postgres)(nil)

// CreateDatabase implements [DB].
func (p *Postgres) CreateDatabase(ctx context.Context, db *sql.DB) error {
	log := log.FromContext(ctx)
	var query string
	if len(p.Template) > 0 {
		log.Info("Creating database with template", "template", p.Template)
		query = fmt.Sprintf("CREATE DATABASE \"%s\" TEMPLATE \"%s\";", p.Database, p.Template)
	} else {
		log.Info("Creating database")
		query = fmt.Sprintf("CREATE DATABASE \"%s\";", p.Database)
	}

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	return nil
}

