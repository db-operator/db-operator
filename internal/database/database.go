package database

import (
	"context"
	"fmt"
)


func Open(
	ctx context.Context,
	cfg Config,
) (DB,error) {


	switch cfg.Engine {

	case Postgres:
		return NewPostgres(ctx,cfg)


	case MySQL:
		return NewMySQL(ctx,cfg)

	case Dummy:
		return NewDummy(ctx,cfg)

	default:
		return nil,fmt.Errorf(
			"unknown database engine %s",
			cfg.Engine,
		)
	}

}
