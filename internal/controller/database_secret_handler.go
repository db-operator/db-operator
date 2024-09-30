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

	kindav1beta1 "github.com/db-operator/db-operator/api/v1beta1"
	"github.com/db-operator/db-operator/pkg/consts"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

/* ------ Secret Event Handler ------ */
type secretEventHandler struct {
	client.Client
}

func (e *secretEventHandler) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	logrus.Info("Start processing Database Secret Update Event")

	switch v := evt.ObjectNew.(type) {

	default:
		logrus.Error("database Secret Update Event error! Unknown object: type=", v.GetObjectKind(), ", name=", evt.ObjectNew.GetNamespace(), "/", evt.ObjectNew.GetName())
		return

	case *corev1.Secret:
		// only labeled secrets are watched
		secretNew := evt.ObjectNew.(*corev1.Secret)
		secretOld := evt.ObjectOld.(*corev1.Secret)

		labels := secretNew.GetLabels()
		kind, ok := labels[consts.USED_BY_KIND_LABEL_KEY]
		if !ok {
			logrus.Errorf("Secret handler won't trigger reconciliation, because %s label is empty", consts.USED_BY_KIND_LABEL_KEY)
			return
		} else if kind != "Database" {
			logrus.Infof("Secret handler won't trigger reconciliation, because %s labels doesn't have value 'Database'", consts.USED_BY_KIND_LABEL_KEY)
		}

		dbcrName, ok := labels[consts.USED_BY_NAME_LABEL_KEY]
		if !ok {
			logrus.Errorf("Secret handler won't trigger reconciliation, because %s label is empty", consts.USED_BY_NAME_LABEL_KEY)
			return
		}

		logrus.Info("processing Database Secret label: name=", consts.USED_BY_NAME_LABEL_KEY, ", value=", dbcrName)

		// send Database Reconcile Request
		dbcr := &kindav1beta1.Database{}
		if err := e.Client.Get(ctx, types.NamespacedName{Namespace: secretNew.GetNamespace(), Name: dbcrName}, dbcr); err != nil {
			logrus.Errorf("couldn't get the database resource: %s - %s", secretNew.GetNamespace(), dbcrName)
			return
		}

		if dbcr.IsDeleted() {
			logrus.Warnf("database %s has been marked for deletion, reconciliation won't be triggered", dbcrName)
			return
		}

		// By default we don't need to run full reconciliation
		fullReconcile := false

		if _, ok := secretNew.GetAnnotations()[consts.SECRET_FORCE_RECONCILE]; ok {
			fullReconcile = true
			defer func() {
				delete(secretNew.Annotations, consts.SECRET_FORCE_RECONCILE)
				logrus.Infof("removing annotation %s from the seecret %s", consts.SECRET_FORCE_RECONCILE, secretNew.GetName())
				if err := e.Client.Update(ctx, secretNew, &client.UpdateOptions{}); err != nil {
					logrus.Errorf("couldn't remove annotation: %s", err)
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

			default:
				logrus.Errorf("unknown database engine: %s", dbcr.Status.Engine)
			}

			for _, key := range inputsKeys {
				if !bytes.Equal(secretNew.Data[key], secretOld.Data[key]) {
					fullReconcile = true
					break
				}
			}
		}

		if fullReconcile {
			logrus.Info("database Secret has been changed and related database resource will be reconciled: secret=", secretNew.Namespace, "/", secretNew.Name, ", database=", dbcrName)
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: secretNew.GetNamespace(),
				Name:      dbcrName,
			}})
		}
	}
}

func (e *secretEventHandler) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	logrus.Error("secretEventHandler.Delete(...) event has been FIRED but NOT implemented!")
}

func (e *secretEventHandler) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	logrus.Error("secretEventHandler.Generic(...) event has been FIRED but NOT implemented!")
}

func (e *secretEventHandler) Create(ctx context.Context, evt event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	logrus.Error("secretEventHandler.Create(...) event has been FIRED but NOT implemented!")
}

/* ------ Event Filter Functions ------ */

func isWatchedNamespace(watchNamespaces []string, ro runtime.Object) bool {
	if watchNamespaces[0] == "" { // # it's necessary to set "" to watch cluster wide
		return true // watch for all namespaces
	}
	// define object's namespace
	objectNamespace := ""
	database, isDatabase := ro.(*kindav1beta1.Database)
	if isDatabase {
		objectNamespace = database.Namespace
	} else {
		secret, isSecret := ro.(*corev1.Secret)
		if isSecret {
			objectNamespace = secret.Namespace
		} else {
			logrus.Info("unknown object", "object", ro)
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

func isDatabase(ro runtime.Object) bool {
	_, isDatabase := ro.(*kindav1beta1.Database)
	return isDatabase
}

func isObjectUpdated(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		logrus.Error(nil, "Update event has no old runtime object to update", "event", e)
		return false
	}
	if e.ObjectNew == nil {
		logrus.Error(nil, "Update event has no new runtime object for update", "event", e)
		return false
	}
	// if object kind is a Database check that 'metadata.generation' field ('spec' section) has been changed
	_, isDatabase := e.ObjectNew.(*kindav1beta1.Database)
	if isDatabase {
		return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration()
	}

	// if object kind is a Secret check that password value has changed
	secretNew, isSecret := e.ObjectNew.(*corev1.Secret)
	if isSecret {
		// only labeled secrets are watched
		labels := secretNew.ObjectMeta.GetLabels()
		dbcrName, ok := labels[consts.USED_BY_NAME_LABEL_KEY]
		if !ok {
			return false // no label found
		}
		logrus.Info("Secret Update Event detected: secret=", secretNew.Namespace, "/", secretNew.Name, ", database=", dbcrName)
		return true
	}
	return false // unknown object, ignore Update Event
}
