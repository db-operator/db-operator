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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	promDBsStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "db_operator",
		Subsystem: "database",
		Name:      "status",
		Help:      "Return information about the status of a database (cr) object",
	},
		[]string{
			"db_namespace",
			"dbinstance",
			"database",
		})

	promDBsPhaseTime = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  "db_operator",
		Subsystem:  "handler",
		Name:       "database_seconds",
		Help:       "Return Summary over the internal timing of the phase functions of the database object",
		Objectives: map[float64]float64{},
	},
		[]string{
			"phase",
		})

	promDBsPhaseError = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "db_operator",
		Subsystem: "handler",
		Name:      "database_phase_error",
		Help:      "Count errors in reconcile cycle",
	},
		[]string{
			"phase",
		})
	promDBInstancesPhase = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "db_operator",
		Subsystem: "dbinstance",
		Name:      "phase",
		Help:      "Return information about the phase of a database instances (cr) object",
	},
		[]string{
			"dbinstance",
		})
	promDBInstancesPhaseTime = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  "db_operator",
		Subsystem:  "handler",
		Name:       "dbinstance_seconds",
		Help:       "Return Summary over the internal timing of the phase of the database instance object",
		Objectives: map[float64]float64{},
	},
		[]string{
			"phase",
		})
)

func boolToFloat64(b bool) float64 {
	if b {
		return float64(1)
	}
	return float64(0)
}
