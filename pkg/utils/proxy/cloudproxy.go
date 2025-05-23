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

package proxy

import (
	"fmt"
	"strconv"

	"github.com/db-operator/db-operator/v2/pkg/config"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// CloudProxy for google sql instance
type CloudProxy struct {
	NamePrefix             string
	Namespace              string
	InstanceConnectionName string
	AccessSecretName       string
	Engine                 string
	Port                   int32
	Labels                 map[string]string
	Conf                   *config.Config
	MonitoringEnabled      bool
}

const instanceAccessSecretVolumeName string = "gcloud-secret"

func (cp *CloudProxy) buildService() (*v1.Service, error) {
	return &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cp.NamePrefix + "-svc",
			Namespace: cp.Namespace,
			Labels:    cp.Labels,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:       cp.Engine,
					Port:       cp.Port,
					TargetPort: intstr.FromString("sqlport"),
					Protocol:   v1.ProtocolTCP,
				},
				{
					Name:       "metrics",
					Port:       int32(cp.Conf.Instances.Google.ProxyConfig.MetricsPort),
					TargetPort: intstr.FromString("metrics"),
					Protocol:   v1.ProtocolTCP,
				},
			},
			Selector: cp.Labels,
		},
	}, nil
}

func (cp *CloudProxy) buildDeployment() (*v1apps.Deployment, error) {
	spec, err := cp.deploymentSpec()
	if err != nil {
		return nil, err
	}

	return &v1apps.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cp.NamePrefix + "-cloudproxy",
			Namespace: cp.Namespace,
			Labels:    cp.Labels,
		},
		Spec: spec,
	}, nil
}

func (cp *CloudProxy) deploymentSpec() (v1apps.DeploymentSpec, error) {
	var replicas int32 = 2

	container, err := cp.container()
	if err != nil {
		return v1apps.DeploymentSpec{}, err
	}

	volumes := []v1.Volume{
		{
			Name: instanceAccessSecretVolumeName,
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: cp.AccessSecretName,
				},
			},
		},
	}

	terminationGracePeriodSeconds := int64(120) // force kill pod after this time

	var annotations map[string]string

	if cp.MonitoringEnabled {
		annotations = map[string]string{
			"prometheus.io/scrape": "true",
			"prometheus.io/port":   strconv.Itoa(cp.Conf.Instances.Google.ProxyConfig.MetricsPort),
		}
	}

	return v1apps.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: cp.Labels,
		},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: annotations,
				Labels:      cp.Labels,
			},
			Spec: v1.PodSpec{
				Containers:    []v1.Container{container},
				NodeSelector:  cp.Conf.Instances.Google.ProxyConfig.NodeSelector,
				RestartPolicy: v1.RestartPolicyAlways,
				Volumes:       volumes,
				Affinity: &v1.Affinity{
					PodAntiAffinity: podAntiAffinity(cp.Labels),
				},
				TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			},
		},
	}, nil
}

func (cp *CloudProxy) container() (v1.Container, error) {
	RunAsUser := int64(2)
	AllowPrivilegeEscalation := false
	listenArg := fmt.Sprintf("--listen=0.0.0.0:%d", cp.Port)
	instanceArg := fmt.Sprintf("--instance=%s", cp.InstanceConnectionName)

	return v1.Container{
		Name:    "db-auth-gateway",
		Image:   cp.Conf.Instances.Google.ProxyConfig.Image,
		Command: []string{"/usr/local/bin/db-auth-gateway"},
		Args:    []string{"--credential-file=/srv/gcloud/credentials.json", listenArg, instanceArg},
		SecurityContext: &v1.SecurityContext{
			RunAsUser:                &RunAsUser,
			AllowPrivilegeEscalation: &AllowPrivilegeEscalation,
		},
		ImagePullPolicy: v1.PullIfNotPresent,
		Ports: []v1.ContainerPort{
			{
				Name:          "sqlport",
				ContainerPort: cp.Port,
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "metrics",
				ContainerPort: int32(cp.Conf.Instances.Google.ProxyConfig.MetricsPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      instanceAccessSecretVolumeName,
				MountPath: "/srv/gcloud/",
			},
		},
	}, nil
}

func (cp *CloudProxy) buildConfigMap() (*v1.ConfigMap, error) {
	return nil, nil
}

func (cp *CloudProxy) buildServiceMonitor() (*promv1.ServiceMonitor, error) {
	Endpoint := promv1.Endpoint{
		Port: "metrics",
	}

	return &promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cp.NamePrefix + "-sm",
			Namespace: cp.Namespace,
			Labels:    cp.Labels,
		},
		Spec: promv1.ServiceMonitorSpec{
			Endpoints: []promv1.Endpoint{Endpoint},
			NamespaceSelector: promv1.NamespaceSelector{
				MatchNames: []string{cp.Namespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: cp.Labels,
			},
		},
	}, nil
}
