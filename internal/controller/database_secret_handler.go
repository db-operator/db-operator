/*
 * Copyright 2021 kloeckner.i GmbH
 * Copyright 2023 DB-Operator Authors
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

package controllers

import (
	"bytes"
	"context"

	kindav1beta2 "github.com/db-operator/db-operator/v2/api/v1beta2"
	"github.com/db-operator/db-operator/v2/pkg/consts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

/* ------ Secret Event Handler ------ */
type secretEventHandler struct {
	client.Client
}

func (e *secretEventHandler) Update(ctx context.Context, evt event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	log := ctrllog.FromContext(ctx)
	log.Info("Start processing Database Secret Update Event")

	if evt.ObjectNew == nil || evt.ObjectOld == nil {
		log.V(1).Info("ignoring secret update event with nil object")
		return
	}

	secretNew, ok := evt.ObjectNew.(*corev1.Secret)
	if !ok {
		log.V(1).Info("ignoring update event where new object is not a Secret")
		return
	}

	secretOld, ok := evt.ObjectOld.(*corev1.Secret)
	if !ok {
		log.V(1).Info("ignoring update event where old object is not a Secret")
		return
	}

	labels := secretNew.GetLabels()
	kind, ok := labels[consts.USED_BY_KIND_LABEL_KEY]
	if !ok {
		log.V(1).Info("Secret handler won't trigger reconciliation, because label is empty", "label", consts.USED_BY_KIND_LABEL_KEY)
		return
	} else if kind != "Database" {
		log.V(1).Info("Secret handler won't trigger reconciliation, because label doesn't have value 'Database'", "label", consts.USED_BY_KIND_LABEL_KEY)
		return
	}

	dbcrName, ok := labels[consts.USED_BY_NAME_LABEL_KEY]
	if !ok {
		log.V(1).Info("Secret handler won't trigger reconciliation, because label is empty", "label", consts.USED_BY_NAME_LABEL_KEY)
		return
	}

	log.Info("processing Database Secret label", "name", consts.USED_BY_NAME_LABEL_KEY, "value", dbcrName)

	// send Database Reconcile Request
	dbcr := &kindav1beta2.Database{}
	if err := e.Get(ctx, types.NamespacedName{Namespace: secretNew.GetNamespace(), Name: dbcrName}, dbcr); err != nil {
		log.Error(err, "couldn't get the database resource", "namespace", secretNew.GetNamespace(), "name", dbcrName)
		return
	}

	if dbcr.IsDeleted() {
		log.Info("database has been marked for deletion, reconciliation won't be triggered", "name", dbcrName)
		return
	}

	// By default we don't need to run full reconciliation
	fullReconcile := false

	if _, ok := secretNew.GetAnnotations()[consts.SECRET_FORCE_RECONCILE]; ok {
		fullReconcile = true
		defer func() {
			delete(secretNew.Annotations, consts.SECRET_FORCE_RECONCILE)
			log.Info("removing annotation from the secret", "annotation", consts.SECRET_FORCE_RECONCILE, "secret", secretNew.GetName())
			if err := e.Client.Update(ctx, secretNew, &client.UpdateOptions{}); err != nil {
				log.Error(err, "couldn't remove annotation")
			}
		}()
	} else {
		inputsKeys := []string{}
		switch dbcr.Status.Engine {
		case "postgres":
			inputsKeys = []string{
				consts.POSTGRES_DB,
				consts.POSTGRES_PASSWORD,
				consts.POSTGRES_USER,
			}

		case "mysql":
			inputsKeys = []string{
				consts.MYSQL_DB,
				consts.MYSQL_PASSWORD,
				consts.MYSQL_USER,
			}

		case "clickhouse":
			inputsKeys = []string{
				consts.CLICKHOUSE_DB,
				consts.CLICKHOUSE_PASSWORD,
				consts.CLICKHOUSE_USER,
			}

		default:
			log.Info("unknown database engine", "engine", dbcr.Status.Engine)
		}

		for _, key := range inputsKeys {
			if !bytes.Equal(secretNew.Data[key], secretOld.Data[key]) {
				fullReconcile = true
				break
			}
		}
	}

	if fullReconcile {
		log.Info("database Secret has been changed and related database resource will be reconciled", "secret", secretNew.Namespace+"/"+secretNew.Name, "database", dbcrName)
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: secretNew.GetNamespace(),
			Name:      dbcrName,
		}})
	}
}

func (e *secretEventHandler) Delete(_ context.Context, _ event.TypedDeleteEvent[client.Object], _ workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	ctrllog.Log.V(1).Info("ignoring delete event for database secret handler")
}

func (e *secretEventHandler) Generic(_ context.Context, _ event.TypedGenericEvent[client.Object], _ workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	ctrllog.Log.V(1).Info("ignoring generic event for database secret handler")
}

func (e *secretEventHandler) Create(_ context.Context, _ event.TypedCreateEvent[client.Object], _ workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	ctrllog.Log.V(1).Info("ignoring create event for database secret handler")
}

/* ------ Event Filter Functions ------ */

func isWatchedNamespace(watchNamespaces []string, ro client.Object) bool {
	if len(watchNamespaces) == 0 || watchNamespaces[0] == "" { // # it's necessary to set "" to watch cluster wide
		return true // watch for all namespaces
	}
	// define object's namespace
	objectNamespace := ""
	database, isDatabase := ro.(*kindav1beta2.Database)
	if isDatabase {
		objectNamespace = database.Namespace
	} else {
		secret, isSecret := ro.(*corev1.Secret)
		if isSecret {
			objectNamespace = secret.Namespace
		} else {
			ctrllog.Log.Info("unknown object", "object", ro)
			return false
		}
	}

	// check that current namespace is watched by db-operator
	for _, ns := range watchNamespaces {
		if ns == objectNamespace {
			return true
		}
	}
	return false
}

func isDatabase(ro client.Object) bool {
	_, isDatabase := ro.(*kindav1beta2.Database)
	return isDatabase
}

func isObjectUpdated(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		ctrllog.Log.Error(nil, "Update event has no old runtime object to update", "event", e)
		return false
	}
	if e.ObjectNew == nil {
		ctrllog.Log.Error(nil, "Update event has no new runtime object for update", "event", e)
		return false
	}
	// if object kind is a Database check that 'metadata.generation' field ('spec' section) has been changed
	_, isDatabase := e.ObjectNew.(*kindav1beta2.Database)
	if isDatabase {
		return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration()
	}

	// if object kind is a Secret check that password value has changed
	secretNew, isSecret := e.ObjectNew.(*corev1.Secret)
	if isSecret {
		// only labeled secrets are watched
		labels := secretNew.GetLabels()
		dbcrName, ok := labels[consts.USED_BY_NAME_LABEL_KEY]
		if !ok {
			return false // no label found
		}
		ctrllog.Log.Info("Secret Update Event detected", "secret", secretNew.Namespace+"/"+secretNew.Name, "database", dbcrName)
		return true
	}
	return false // unknown object, ignore Update Event
}
