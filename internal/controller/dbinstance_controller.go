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

package controllers

import (
	"context"
	kindav1 "github.com/db-operator/db-operator/v2/api/v1"
	kubehelper "github.com/db-operator/db-operator/v2/internal/helpers/kube"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type DbInstanceReconcilerOpts struct {
	ReconcileInterval time.Duration
}

// DbInstanceReconciler reconciles a DbInstance object
type DbInstanceReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	kubeHelper *kubehelper.KubeHelper
	Opts       *DbInstanceReconcilerOpts
}

//+kubebuilder:rbac:groups=kinda.rocks,resources=dbinstances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kinda.rocks,resources=dbinstances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kinda.rocks,resources=dbinstances/finalizers,verbs=update

func (r *DbInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	reconcilePeriod := r.Opts.ReconcileInterval * time.Second
	reconcileResult := reconcile.Result{RequeueAfter: reconcilePeriod}

	// Fetch the DbInstance custom resource
	dbin := &kindav1.DbInstance{}
	err := r.Get(ctx, req.NamespacedName, dbin)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcileResult, nil
		}
		// Error reading the object - requeue the request.
		return reconcileResult, err
	}

	// Update object status always when function returns, either normally or through a panic.
	defer func() {
		if err := r.Status().Update(ctx, dbin); err != nil {
			log.Error(err, "failed to update status")
		}
	}()

	mustReconfile := false
	r.kubeHelper = kubehelper.NewKubeHelper(r.Client, r.Recorder, dbin)

	if mustReconfile {
		log.Info("Must reconcile")
	}

	return reconcileResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kindav1.DbInstance{}).
		Complete(r)
}
