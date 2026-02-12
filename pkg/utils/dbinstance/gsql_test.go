package dbinstance

import (
	"context"
	"errors"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (ins *Gsql) mockWaitUntilRunnable(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.V(2).Info("waiting gsql instance", ins.Name)

	time.Sleep(10 * time.Second)

	state, err := ins.state()
	if err != nil {
		return err
	}
	if state != "RUNNABLE" {
		return errors.New("gsql instance not ready yet")
	}

	return nil
}

func mockGsqlConfig() string {
	return `{
		"databaseVersion": "POSTGRES_12",
		"settings": {
		  "tier": "db-f1-micro",
		  "availabilityType": "ZONAL",
		  "pricingPlan": "PER_USE",
		  "replicationType": "SYNCHRONOUS",
		  "activationPolicy": "ALWAYS",
		  "ipConfiguration": {
			"authorizedNetworks": [],
			"ipv4Enabled": true
		  },
		  "dataDiskType": "PD_SSD",
		  "backupConfiguration": {
			"enabled": false
		  },
		  "storageAutoResizeLimit": "0",
		  "storageAutoResize": true
		},
		"backendType": "SECOND_GEN",
		"region": "somewhere"
}`
}

func myMockGsql() *Gsql {
	return &Gsql{
		Name:        uuid.New().String(),
		ProjectID:   "test-project",
		APIEndpoint: "http://127.0.0.1:8080",
		Config:      mockGsqlConfig(),
		User:        "test-user1",
		Password:    "testPassw0rd",
	}
}

func TestGsqlGetInstanceNonExist(ctx context.Context, t *testing.T) {
	log := log.FromContext(ctx)
	myGsql := myMockGsql()

	rs, err := myGsql.getInstance()
	log.Info("error", err, rs)
	assert.Error(t, err)
}

func TestGsqlCreateInvalidInstance(ctx context.Context, t *testing.T) {
	myGsql := myMockGsql()
	myGsql.Config = ""

	err := myGsql.createInstance()
	assert.Error(t, err)
}

func TestGsqlCreateInstance(ctx context.Context, t *testing.T) {
	myGsql := myMockGsql()

	patchWait := monkey.Patch((*Gsql).waitUntilRunnable, (*Gsql).mockWaitUntilRunnable)
	defer patchWait.Unpatch()

	err := myGsql.createInstance()
	assert.NoError(t, err)
}

func TestGsqlGetInstanceExist(ctx context.Context, t *testing.T) {
	log := log.FromContext(ctx)
	myGsql := myMockGsql()

	err := myGsql.createInstance()
	assert.NoError(t, err)

	rs, err := myGsql.getInstance()
	log.Info("error", err, rs)
	assert.NoError(t, err)
}

func TestGsqlCreateExistingInstance(ctx context.Context, t *testing.T) {
	myGsql := myMockGsql()

	err := myGsql.createInstance()
	assert.NoError(t, err)

	err = myGsql.createInstance()
	assert.Error(t, err)
}

func TestGsqlUpdateInstance(ctx context.Context, t *testing.T) {
	myGsql := myMockGsql()

	err := myGsql.createInstance()
	assert.NoError(t, err)

	patchWait := monkey.Patch((*Gsql).waitUntilRunnable, (*Gsql).mockWaitUntilRunnable)
	defer patchWait.Unpatch()

	err = myGsql.updateInstance()
	assert.NoError(t, err)
}

func TestGsqlUpdateUser(ctx context.Context, t *testing.T) {
	myGsql := myMockGsql()

	err := myGsql.createInstance()
	assert.NoError(t, err)

	err = myGsql.updateUser(ctx)
	assert.NoError(t, err)
}
