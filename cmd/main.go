/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	kindarocksv1alpha1 "github.com/db-operator/db-operator/api/v1alpha1"
	kindarocksv1beta1 "github.com/db-operator/db-operator/api/v1beta1"
	controllers "github.com/db-operator/db-operator/internal/controller"
	"github.com/db-operator/db-operator/pkg/config"
	"github.com/db-operator/db-operator/pkg/utils/thirdpartyapi"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.) to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(kindarocksv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kindarocksv1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	thirdpartyapi.AppendToScheme(scheme)
}

func main() {
	var metricsAddr string
	var probeAddr string
	var enableLeaderElection bool
	var checkForChanges bool
	var isWebhook bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":60000", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&checkForChanges, "check-for-changes", false,
		"Enabling this will make the operator only reconcile when k8s objects were changed (currently used only by dbuser crd).")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&isWebhook, "webhook", false, "Starts the webhook server when set.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	webhookSrv := webhook.NewServer(webhook.Options{
		Port: 9443,
	})
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer:          webhookSrv,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "6fe36c14.kinda.rocks",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if isWebhook {
		setupLog.Info("Starting webhook server")

		if err = (&kindarocksv1beta1.Database{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Database")
			os.Exit(1)
		}
		if err = (&kindarocksv1beta1.DbInstance{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "DbInstance")
			os.Exit(1)
		}
		if err = (&kindarocksv1beta1.DbUser{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "DbUser")
			os.Exit(1)
		}
	} else {
		setupLog.Info("Starting controller")
		conf, err := config.LoadConfig()
		if err != nil {
			setupLog.Error(err, "an error occured when reading the config")
			os.Exit(1)
		}

		interval := os.Getenv("RECONCILE_INTERVAL")
		i, err := strconv.ParseInt(interval, 10, 64)
		if err != nil {
			i = 60
			setupLog.Info("Set default reconcile period for database-controller", "time", interval)
		}

		if err = (&controllers.DbInstanceReconciler{
			Client:   mgr.GetClient(),
			Log:      ctrl.Log.WithName("controllers").WithName("DbInstance"),
			Scheme:   mgr.GetScheme(),
			Interval: time.Duration(i),
			Recorder: mgr.GetEventRecorderFor("dbinstance-controller"),
			Conf:     conf,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "DbInstance")
			os.Exit(1)
		}

		watchNamespaces := os.Getenv("WATCH_NAMESPACE")
		namespaces := strings.Split(watchNamespaces, ",")
		setupLog.Info("Database resources will be served in the next namespaces", "namespaces", namespaces)

		if err = (&controllers.DatabaseReconciler{
			Client:          mgr.GetClient(),
			Log:             ctrl.Log.WithName("controllers").WithName("Database"),
			Scheme:          mgr.GetScheme(),
			Recorder:        mgr.GetEventRecorderFor("database-controller"),
			Interval:        time.Duration(i),
			Conf:            conf,
			WatchNamespaces: namespaces,
			CheckChanges:    checkForChanges,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Database")
			os.Exit(1)
		}

		if err = (&controllers.DbUserReconciler{
			Client:       mgr.GetClient(),
			Scheme:       mgr.GetScheme(),
			Recorder:     mgr.GetEventRecorderFor("dbuser-controller"),
			Interval:     time.Duration(i),
			CheckChanges: checkForChanges,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "DbUser")
			os.Exit(1)
		}
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
