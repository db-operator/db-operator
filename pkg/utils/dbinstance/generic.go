/*
 * Copyright 2021 kloeckner.i GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dbinstance

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/db-operator/db-operator/v2/pkg/utils/database"

	"github.com/db-operator/db-operator/v2/pkg/test"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Generic represents database instance which can be connected by address and port
type Generic struct {
	Host         string
	Port         uint16
	Engine       string
	User         string
	Password     string
	PublicIP     string
	SSLEnabled   bool
	SkipCAVerify bool
}

type Engine struct {
	Engine  string
	Version string
}

// DetectEngine detects the engine type of the database instance by trying to connect to it using different engines
func DetectEngine(ctx context.Context, in *Generic, dbuser *database.DatabaseUser) (*Engine, error) {
	// Try postgres first
	log := log.FromContext(ctx)
	log.V(2).Info("detecting engine type for generic db instance")

	detect := func(engine string) (*Engine, error) {
		in.Engine = engine
		log.V(2).Info("trying engine", "engine", engine)

		db, err := MakeInterface(in)
		if err != nil {
			db = nil
			return nil, err
		}

		version, err := db.GetServerVersion(ctx, dbuser)
		if err != nil {
			return nil, err
		}

		return &Engine{Engine: engine, Version: version}, nil
	}

	// If engine is explicitly set, only try that one.
	if in.Engine != "" {
		switch in.Engine {
		case "postgres", "mysql":
			return detect(in.Engine)
		default:
			return nil, fmt.Errorf("unsupported engine %q", in.Engine)
		}
	}

	for _, engine := range []string{"postgres", "mysql"} {
		if detected, err := detect(engine); err == nil {
			return detected, nil
		}
	}

	return nil, errors.New("could not detect engine")
}

func MakeInterface(in *Generic) (database.Database, error) {
	switch in.Engine {
	case "postgres":
		db := database.Postgres{
			Host:         in.Host,
			Port:         in.Port,
			Database:     "postgres",
			SSLEnabled:   in.SSLEnabled,
			SkipCAVerify: in.SkipCAVerify,
		}
		return db, nil
	case "mysql":
		db := database.Mysql{
			Host:         in.Host,
			Port:         in.Port,
			Database:     "mysql",
			SSLEnabled:   in.SSLEnabled,
			SkipCAVerify: in.SkipCAVerify,
		}
		return db, nil
	case "dummy":
		db := database.Dummy{}
		return db, nil
	default:
		return nil, errors.New("not supported engine type")
	}
}

func (ins *Generic) state() (string, error) {
	return "NOT_SUPPORTED", nil
}

func (ins *Generic) exist(ctx context.Context) error {
	log := log.FromContext(ctx)
	db, err := MakeInterface(ins)
	if err != nil {
		log.Error(err, "can not check if instance exists")
		return err
	}
	dbuser := &database.DatabaseUser{
		Username: ins.User,
		Password: ins.Password,
	}

	err = db.CheckStatus(ctx, dbuser)
	if err != nil {
		log.Error(err, "can not check db status")
		return err
	}
	return nil // instance exist
}

func (ins *Generic) create() error {
	return errors.New("creating generic db instance is not yet implemented")
}

func (ins *Generic) update() error {
	return nil
}

func (ins *Generic) getInfoMap() (map[string]string, error) {
	data := map[string]string{
		"DB_CONN":      ins.Host,
		"DB_PORT":      strconv.FormatInt(int64(ins.Port), 10),
		"DB_PUBLIC_IP": ins.PublicIP,
	}

	return data, nil
}

func TestGenericMysqlInstance() *Generic {
	return &Generic{
		Host:     test.GetMysqlHost(),
		Port:     test.GetMysqlPort(),
		Engine:   "mysql",
		User:     "root",
		Password: test.GetMysqlAdminPassword(),
	}
}

func TestGenericPostgresInstance() *Generic {
	return &Generic{
		Host:     test.GetPostgresHost(),
		Port:     test.GetPostgresPort(),
		Engine:   "postgres",
		User:     test.GetPostgresAdminUsername(),
		Password: test.GetPostgresAdminPassword(),
	}
}
