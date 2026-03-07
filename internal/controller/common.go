package controllers

import (
	"context"
	"time"

	kindav1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func handleError(ctx context.Context, err error, object, related client.Object) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	log.Error(err, "An error occurred during the reconciliation")

	r.Recorder.Eventf(object, related, corev1.EventTypeWarning, "ReconcileError", "", issue.Error())
	err := r.Status().Update(ctx, dbcr)
	if err != nil {
		log.Error(err, "unable to update status")
		return reconcile.Result{
			RequeueAfter: retryInterval,
			Requeue:      requeue,
		}, nil
	}

	// TODO: implementing reschedule calculation based on last updated time
	return reconcile.Result{
		RequeueAfter: retryInterval,
		Requeue:      requeue,
	}, nil
}
