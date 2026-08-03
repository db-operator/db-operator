// Copyright 2021 kloeckner.i GmbH
// Copyright 2026 DB-Operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"time"

	kindav1 "github.com/db-operator/db-operator/v2/api/v1"
	kindav1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	"github.com/db-operator/db-operator/v2/internal/controller/helpers"
	commonhelper "github.com/db-operator/db-operator/v2/internal/helpers/common"
	"github.com/db-operator/db-operator/v2/pkg/config"
	"github.com/db-operator/db-operator/v2/pkg/consts"
	"github.com/db-operator/db-operator/v2/pkg/utils/database"
	"github.com/db-operator/db-operator/v2/pkg/utils/dbinstance"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	conditionTypeCredentialsFound = "CredentialsFound"
	conditionTypeEndpointFound    = "EndpointFound"
	conditionTypeHealthy          = "Healthy"
	conditionGrantRulesVerified   = "GrantRulesVerified"
)

// DbInstanceReconciler reconciles a DbInstance object
type DbInstanceReconciler struct {
	client.Client
	Interval time.Duration
	Recorder events.EventRecorder
	Conf     *config.Config
}

//+kubebuilder:rbac:groups=kinda.rocks,resources=dbinstances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kinda.rocks,resources=dbinstances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kinda.rocks,resources=dbinstances/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="events.k8s.io",resources=events,verbs=get;list;watch;update;patch;create

func (r *DbInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling DbInstance")
	reconcilePeriod := r.Interval
	reconcileRequeue := reconcile.Result{RequeueAfter: reconcilePeriod}

	// Fetch the DbInstance custom resource
	dbin := &kindav1.DbInstance{}
	err := r.Get(ctx, req.NamespacedName, dbin)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	original := dbin.DeepCopy()

	// Always patch the resource after the reconcile function.
	defer func() {
		if !reflect.DeepEqual(original.Status, dbin.Status) {
			if err := r.Status().Patch(
				ctx,
				dbin,
				client.MergeFrom(original),
			); err != nil {
				log.Error(err, "failed to update status")
			}
		}
	}()

	dbin.Status.OperatorVersion = commonhelper.OperatorVersion

	// Fetching admin credentials
	dbuser, err := r.fetchCredentials(ctx, dbin)
	if err != nil {
		log.Error(err, "Failed to fetch credentials from source")
		meta.SetStatusCondition(&dbin.Status.Conditions, metav1.Condition{
			Type:    conditionTypeCredentialsFound,
			Status:  metav1.ConditionFalse,
			Reason:  "CredentialsUnavailable",
			Message: fmt.Sprintf("Error occurred while fetching: %s", err.Error()),
		})
		return ctrl.Result{}, err
	}
	meta.SetStatusCondition(&dbin.Status.Conditions, metav1.Condition{
		Type:    conditionTypeCredentialsFound,
		Status:  metav1.ConditionTrue,
		Reason:  "CredentialsAvailable",
		Message: "Successfully fetched credentials",
	})

	// Fetch endpoint data
	genericDB, err := r.fetchEndpoint(ctx, dbin)
	if err != nil {
		log.Error(err, "Failed to fetch endpoint data")
		meta.SetStatusCondition(&dbin.Status.Conditions, metav1.Condition{
			Type:    conditionTypeEndpointFound,
			Status:  metav1.ConditionFalse,
			Reason:  "EndpointDataUnavailable",
			Message: fmt.Sprintf("Error occurred while fetching: %s", err.Error()),
		})
		return ctrl.Result{}, err
	}
	meta.SetStatusCondition(&dbin.Status.Conditions, metav1.Condition{
		Type:    conditionTypeEndpointFound,
		Status:  metav1.ConditionTrue,
		Reason:  "EndpointDataAvailable",
		Message: "Successfully fetched endpoint data",
	})

	dbin.Status.Engine = *dbin.Spec.Engine
	dbin.Status.ServerStatus = &kindav1.DbInstanceServerStatus{}
	dbin.Status.MainEndpoint = &kindav1.DbInstanceServerData{}
	dbin.Status.ReadOnlyEndpoint = &kindav1.DbInstanceServerData{}

	db, err := dbinstance.MakeInterface(genericDB)
	if err != nil {
		log.Error(err, "Failed to create database interface")
		return ctrl.Result{}, err
	}

	if dbin.Status.Version == "" || time.Now().After(time.Unix(dbin.Status.VersionTTL, 10)) {
		dbin.Status.Version, err = db.GetServerVersion(ctx, dbuser)
		if err != nil {
			log.Error(err, "Failed to get server version")
			dbin.Status.Ready = false
			meta.SetStatusCondition(&dbin.Status.Conditions, metav1.Condition{
				Type:    conditionTypeHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  "InstanceNotAvailable",
				Message: fmt.Sprintf("Error occurred while checking instance status: %s", err.Error()),
			})
			return ctrl.Result{}, err
		}
		dbin.Status.VersionTTL = time.Now().Add(r.Conf.ServerVersionTTL).Unix()
	}

	if err := db.CheckStatus(ctx, dbuser); err != nil {
		dbin.Status.Ready = false
		meta.SetStatusCondition(&dbin.Status.Conditions, metav1.Condition{
			Type:    conditionTypeHealthy,
			Status:  metav1.ConditionFalse,
			Reason:  "InstanceNotAvailable",
			Message: fmt.Sprintf("Error occurred while checking instance status: %s", err.Error()),
		})
		return reconcileRequeue, nil
	}

	meta.SetStatusCondition(&dbin.Status.Conditions, metav1.Condition{
		Type:    conditionTypeHealthy,
		Status:  metav1.ConditionTrue,
		Reason:  "InstanceAvailable",
		Message: "Instance is available",
	})

	users, err := db.ListUsers(ctx, dbuser)
	if err != nil {
		log.Error(err, "failed to list users")
		return ctrl.Result{}, err
	}

	if r.Conf.DatabaseAwareness {
		databases, err := db.ListDatabases(ctx, dbuser)
		if err != nil {
			log.Error(err, "Failed to list databases")
			return ctrl.Result{}, err
		}

		count := len(databases)
		dbin.Status.ServerStatus.DatabasesCount = count
		dbin.Status.ServerStatus.Databases = databases
		// We need users for grants, but if the database awareness is disabled
		// we don't store them into crds
		dbin.Status.ServerStatus.Users = users
	}

	if dbin.Spec.GrantRules != nil {
		for _, rule := range dbin.Spec.GrantRules {
			if !slices.Contains(users, rule.Role) {
				err := fmt.Errorf("user %s specified in grant rules does not exist", rule.Role)
				meta.SetStatusCondition(&dbin.Status.Conditions, metav1.Condition{
					Type:    conditionGrantRulesVerified,
					Status:  metav1.ConditionFalse,
					Reason:  "RoleNotFound",
					Message: fmt.Sprintf("Grant roles can't be enabled for missing roles: %s", err.Error()),
				})
				dbin.Status.Ready = false
				log.Error(err, "Grant rule user does not exist")
				return ctrl.Result{}, err
			}
		}
		meta.SetStatusCondition(&dbin.Status.Conditions, metav1.Condition{
			Type:    conditionGrantRulesVerified,
			Status:  metav1.ConditionTrue,
			Reason:  "GrantRulesConfigured",
			Message: "Grant rules verified successfully",
		})
	}

	managedDBCount := 0
	dbList := &kindav1beta1.DatabaseList{}
	if err := r.List(ctx, dbList); err != nil {
		log.Error(err, "Couldn't list databases in the cluster")
		return ctrl.Result{}, err
	}
	for _, db := range dbList.Items {
		if db.Spec.Instance == dbin.Name && db.Status.Status {
			managedDBCount += 1
		}
	}
	dbin.Status.ServerStatus.ManagedDatabasesCount = managedDBCount

	dbin.Status.MainEndpoint.Host = genericDB.Host
	dbin.Status.MainEndpoint.Port = genericDB.Port

	if err := r.labelReferencedResources(ctx, dbin); err != nil {
		log.Error(err, "Failed to label referenced resources")
		return ctrl.Result{}, err
	}

	dbin.Status.NamespaceFilters = []string{}
	if dbin.Spec.NamespaceFilters != nil {
		dbin.Status.NamespaceFilters = dbin.Spec.NamespaceFilters
	}

	dbin.Status.AutoGrantRules = []*kindav1.DbInstanceGrantRule{}
	if dbin.Spec.GrantRules != nil {
		dbin.Status.AutoGrantRules = dbin.Spec.GrantRules
	}

	status := true
	dbin.Status.Ready = status
	if err := r.Client.Status().Update(ctx, dbin); err != nil {
		log.Error(err, "Failed to set DbInstance status to ready")
		return ctrl.Result{}, err
	}

	return reconcileRequeue, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kindav1.DbInstance{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findDbInstanceForResource),
		).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.findDbInstanceForResource),
		).
		Complete(r)
}

func (r *DbInstanceReconciler) findDbInstanceForResource(ctx context.Context, obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	if dbInstanceName, ok := labels[consts.DBINSTANCE_NAME_LABEL_KEY]; ok {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name: dbInstanceName,
				},
			},
		}
	}
	return nil
}

// labelReferencedResources adds a label to all resources referenced by the DbInstance, allowing for easy identification and management of related resources.
func (r *DbInstanceReconciler) labelReferencedResources(ctx context.Context, dbin *kindav1.DbInstance) error {
	log := log.FromContext(ctx)
	currentlyWatchedResources := []string{}
	authData := dbin.Spec.Auth
	if authData == nil {
		return errors.New("auth data is nil")
	}
	endpointData := dbin.Spec.Endpoint
	if endpointData == nil {
		return errors.New("endpoint data is nil")
	}
	log.Info("Labeling referenced resources for DbInstance")

	referencedResources := []*kindav1.ValueFrom{}
	if authData.Username != nil && authData.Username.ValueFrom != nil {
		referencedResources = append(referencedResources, authData.Username.ValueFrom)
	}
	if authData.Password != nil && authData.Password.ValueFrom != nil {
		referencedResources = append(referencedResources, authData.Password.ValueFrom)
	}
	if endpointData.Host != nil && endpointData.Host.ValueFrom != nil {
		referencedResources = append(referencedResources, endpointData.Host.ValueFrom)
	}
	if endpointData.Port != nil && endpointData.Port.ValueFrom != nil {
		referencedResources = append(referencedResources, endpointData.Port.ValueFrom)
	}

	for _, resource := range referencedResources {
		obj, err := helpers.GetResourceFromValueSource(ctx, r.Client, resource)
		if err != nil {
			return err
		}
		if err := commonhelper.EnsureLabel(ctx, r.Client, obj, consts.DBINSTANCE_NAME_LABEL_KEY, dbin.Name); err != nil {
			return err
		}
		objEntry := helpers.ObjectMetadataFormat(obj)

		if !slices.Contains(currentlyWatchedResources, objEntry) {
			currentlyWatchedResources = append(currentlyWatchedResources, objEntry)
		}
	}

	current := make(map[string]struct{}, len(currentlyWatchedResources))
	for _, e := range currentlyWatchedResources {
		current[e] = struct{}{}
	}

	var stale []string
	for _, e := range dbin.Status.WatchedResources {
		if _, ok := current[e]; !ok {
			stale = append(stale, e)
		}
	}

	for _, resourceEntry := range stale {
		obj, err := helpers.ObjectFromFormattedString(resourceEntry)
		if err != nil {
			return err
		}
		if err := commonhelper.EnsureLabelRemoved(ctx, r.Client, obj, consts.DBINSTANCE_NAME_LABEL_KEY, dbin.Name); err != nil {
			return err
		}
	}

	dbin.Status.WatchedResources = currentlyWatchedResources
	return nil
}

// fetchCredentials retrieves the database credentials from the specified sources in the DbInstance spec.
func (r *DbInstanceReconciler) fetchCredentials(ctx context.Context, dbin *kindav1.DbInstance) (*database.DatabaseUser, error) {
	log := log.FromContext(ctx)
	log.V(2).Info("Fetching credentials from source")

	username, err := helpers.GetValueFromSource(ctx, r.Client, dbin.Spec.Auth.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch username from source: %w", err)
	}

	password, err := helpers.GetValueFromSource(ctx, r.Client, dbin.Spec.Auth.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch password from source: %w", err)
	}

	dbuser := &database.DatabaseUser{
		Username: username,
		Password: password,
	}

	return dbuser, nil
}

func (r *DbInstanceReconciler) fetchEndpoint(ctx context.Context, dbin *kindav1.DbInstance) (*dbinstance.Generic, error) {
	log := log.FromContext(ctx)
	log.V(2).Info("Fetching host and port from source")

	host, err := helpers.GetValueFromSource(ctx, r.Client, dbin.Spec.Endpoint.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch host from source: %w", err)
	}

	portRaw, err := helpers.GetValueFromSource(ctx, r.Client, dbin.Spec.Endpoint.Port)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch port from source: %w", err)
	}
	portInt, err := strconv.Atoi(portRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to convert port to integer: %w", err)
	}
	port := uint16(portInt)

	// Prepare SSL Connection
	sslConnecton := dbin.Spec.Endpoint.SSLConnection
	if sslConnecton == nil {
		sslConnecton = &kindav1.DbInstanceSSLConnection{
			Enabled:    false,
			SkipVerify: false,
		}
	}

	genericDB := &dbinstance.Generic{
		Host:         host,
		Port:         port,
		SSLEnabled:   sslConnecton.Enabled,
		SkipCAVerify: sslConnecton.SkipVerify,
		Engine:       *dbin.Spec.Engine,
	}

	if dbin.Spec.Engine != nil {
		genericDB.Engine = *dbin.Spec.Engine
	}

	return genericDB, nil
}
