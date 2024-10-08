package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestDeploymentTemplate(t *testing.T) {
	for _, tc := range []struct {
		CaseName       string
		Release        string
		Values         map[string]string
		ExpectedLabels map[string]string

		ExpectedErrorRegexp *regexp.Regexp

		ExpectedName         string
		ExpectedRelease      string
		ExpectedStrategyType appsV1.DeploymentStrategyType
	}{
		{
			CaseName: "happy",
			Release:  "production",
			Values: map[string]string{
				"releaseOverride": "productionOverridden",
			},
			ExpectedName:         "productionOverridden",
			ExpectedRelease:      "production",
			ExpectedStrategyType: appsV1.DeploymentStrategyType(""),
			ExpectedLabels:       nil,
		},
		{
			CaseName: "extraLabel",
			Release:  "production",
			Values: map[string]string{
				"releaseOverride":        "productionOverridden",
				"extraLabels.firstLabel": "expected-label",
			},
			ExpectedName:         "productionOverridden",
			ExpectedRelease:      "production",
			ExpectedStrategyType: appsV1.DeploymentStrategyType(""),
			ExpectedLabels: map[string]string{
				"firstLabel": "expected-label",
			},
		},
		{
			// See https://github.com/helm/helm/issues/6006
			CaseName: "long release name",
			Release:  strings.Repeat("r", 80),

			ExpectedErrorRegexp: regexp.MustCompile("Error: release name .* length must not be longer than 53"),
			ExpectedLabels:      nil,
		},
		{
			CaseName: "strategyType",
			Release:  "production",
			Values: map[string]string{
				"strategyType": "Recreate",
			},
			ExpectedName:         "production",
			ExpectedRelease:      "production",
			ExpectedStrategyType: appsV1.RecreateDeploymentStrategyType,
			ExpectedLabels:       nil,
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, tc.ExpectedErrorRegexp)

			if tc.ExpectedErrorRegexp != nil {
				return
			}

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedName, deployment.Name)
			require.Equal(t, tc.ExpectedStrategyType, deployment.Spec.Strategy.Type)

			require.Equal(t, map[string]string{
				"app.gitlab.com/app": "auto-devops-examples/minimal-ruby-app",
				"app.gitlab.com/env": "prod",
			}, deployment.Annotations)

			ExpectedLabels := map[string]string{
				"app":                          tc.ExpectedName,
				"chart":                        chartName,
				"heritage":                     "Helm",
				"release":                      tc.ExpectedRelease,
				"tier":                         "web",
				"track":                        "stable",
				"app.kubernetes.io/name":       tc.ExpectedName,
				"helm.sh/chart":                chartName,
				"app.kubernetes.io/managed-by": "Helm",
				"app.kubernetes.io/instance":   tc.ExpectedRelease,
			}
			mergeStringMap(ExpectedLabels, tc.ExpectedLabels)

			require.Equal(t, ExpectedLabels, deployment.Labels)

			require.Equal(t, map[string]string{
				"app.gitlab.com/app":           "auto-devops-examples/minimal-ruby-app",
				"app.gitlab.com/env":           "prod",
				"checksum/application-secrets": "",
			}, deployment.Spec.Template.Annotations)
			require.Equal(t, ExpectedLabels, deployment.Spec.Template.Labels)
		})
	}

	for _, tc := range []struct {
		CaseName                string
		Release                 string
		Values                  map[string]string
		ExpectedImageRepository string
	}{
		{
			CaseName: "skaffold",
			Release:  "production",
			Values: map[string]string{
				"image.repository": "skaffold",
				"image.tag":        "",
			},
			ExpectedImageRepository: "skaffold",
		},
		{
			CaseName: "skaffold",
			Release:  "production",
			Values: map[string]string{
				"image.repository": "skaffold",
				"image.tag":        "stable",
			},
			ExpectedImageRepository: "skaffold:stable",
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedImageRepository, deployment.Spec.Template.Spec.Containers[0].Image)
		})
	}

	for _, tc := range []struct {
		CaseName        string
		Release         string
		Values          map[string]string
		ExpectedCommand []string
		ExpectedArgs    []string
	}{
		{
			CaseName: "application-command",
			Release:  "production",
			Values: map[string]string{
				"application.command[0]": "foo",
				"application.command[1]": "bar",
				"application.command[2]": "baz",
			},
			ExpectedCommand: []string{"foo", "bar", "baz"},
		},
		{
			CaseName: "application-args",
			Release:  "production",
			Values: map[string]string{
				"application.args[0]": "foo",
				"application.args[1]": "bar",
				"application.args[2]": "baz",
			},
			ExpectedArgs: []string{"foo", "bar", "baz"},
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedCommand, deployment.Spec.Template.Spec.Containers[0].Command)
			require.Equal(t, tc.ExpectedArgs, deployment.Spec.Template.Spec.Containers[0].Args)
		})
	}

	for _, tc := range []struct {
		CaseName            string
		Release             string
		Values              map[string]string
		ExpectedHostNetwork bool
	}{
		{
			CaseName: "root hostNetwork is defined",
			Release:  "production",
			Values: map[string]string{
				"hostNetwork": "true",
			},
			ExpectedHostNetwork: bool(true),
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedHostNetwork, deployment.Spec.Template.Spec.HostNetwork)
		})
	}

	// ImagePullSecrets
	for _, tc := range []struct {
		CaseName                 string
		Release                  string
		Values                   map[string]string
		ExpectedImagePullSecrets []coreV1.LocalObjectReference
	}{
		{
			CaseName: "default secret",
			Release:  "production",
			Values:   map[string]string{},
			ExpectedImagePullSecrets: []coreV1.LocalObjectReference{
				{
					Name: "gitlab-registry",
				},
			},
		},
		{
			CaseName: "present secret",
			Release:  "production",
			Values: map[string]string{
				"image.secrets[0].name": "expected-secret",
			},
			ExpectedImagePullSecrets: []coreV1.LocalObjectReference{
				{
					Name: "expected-secret",
				},
			},
		},
		{
			CaseName: "multiple secrets",
			Release:  "production",
			Values: map[string]string{
				"image.secrets[0].name": "expected-secret",
				"image.secrets[1].name": "additional-secret",
			},
			ExpectedImagePullSecrets: []coreV1.LocalObjectReference{
				{
					Name: "expected-secret",
				},
				{
					Name: "additional-secret",
				},
			},
		},
		{
			CaseName: "missing secret",
			Release:  "production",
			Values: map[string]string{
				"image.secrets": "null",
			},
			ExpectedImagePullSecrets: nil,
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedImagePullSecrets, deployment.Spec.Template.Spec.ImagePullSecrets)
		})
	}

	// podAnnotations
	for _, tc := range []struct {
		CaseName               string
		Values                 map[string]string
		Release                string
		ExpectedPodAnnotations map[string]string
	}{
		{
			CaseName: "one podAnnotations",
			Release:  "production",
			Values: map[string]string{
				"podAnnotations.firstAnnotation": "expected-annotation",
			},
			ExpectedPodAnnotations: map[string]string{
				"checksum/application-secrets": "",
				"app.gitlab.com/app":           "auto-devops-examples/minimal-ruby-app",
				"app.gitlab.com/env":           "prod",
				"firstAnnotation":              "expected-annotation",
			},
		},
		{
			CaseName: "multiple podAnnotations",
			Release:  "production",
			Values: map[string]string{
				"podAnnotations.firstAnnotation":  "expected-annotation",
				"podAnnotations.secondAnnotation": "expected-annotation",
			},
			ExpectedPodAnnotations: map[string]string{
				"checksum/application-secrets": "",
				"app.gitlab.com/app":           "auto-devops-examples/minimal-ruby-app",
				"app.gitlab.com/env":           "prod",
				"firstAnnotation":              "expected-annotation",
				"secondAnnotation":             "expected-annotation",
			},
		},
		{
			CaseName: "no podAnnotations",
			Release:  "production",
			Values:   map[string]string{},
			ExpectedPodAnnotations: map[string]string{
				"checksum/application-secrets": "",
				"app.gitlab.com/app":           "auto-devops-examples/minimal-ruby-app",
				"app.gitlab.com/env":           "prod",
			},
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedPodAnnotations, deployment.Spec.Template.ObjectMeta.Annotations)
		})
	}

	// serviceAccountName
	for _, tc := range []struct {
		CaseName                   string
		Release                    string
		Values                     map[string]string
		ExpectedServiceAccountName string
	}{
		{
			CaseName:                   "default service account",
			Release:                    "production",
			ExpectedServiceAccountName: "",
		},
		{
			CaseName: "empty service account name",
			Release:  "production",
			Values: map[string]string{
				"serviceAccountName": "",
			},
			ExpectedServiceAccountName: "",
		},
		{
			CaseName: "custom service account name - myServiceAccount",
			Release:  "production",
			Values: map[string]string{
				"serviceAccountName": "myServiceAccount",
			},
			ExpectedServiceAccountName: "myServiceAccount",
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)
		})
	}

	// serviceAccount
	for _, tc := range []struct {
		CaseName string
		Release  string
		Values   map[string]string

		ExpectedServiceAccountName string
	}{
		{
			CaseName:                   "default service account",
			Release:                    "production",
			ExpectedServiceAccountName: "",
		},
		{
			CaseName: "empty service account name",
			Release:  "production",
			Values: map[string]string{
				"serviceAccount.name": "",
			},
			ExpectedServiceAccountName: "",
		},
		{
			CaseName: "custom service account name - myServiceAccount",
			Release:  "production",
			Values: map[string]string{
				"serviceAccount.name": "myServiceAccount",
			},
			ExpectedServiceAccountName: "myServiceAccount",
		},
		{
			CaseName: "serviceAccount.name takes precedence over serviceAccountName",
			Release:  "production",
			Values: map[string]string{
				"serviceAccount.name": "myServiceAccount1",
				"serviceAccountName":  "myServiceAccount2",
			},
			ExpectedServiceAccountName: "myServiceAccount1",
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)
		})
	}

	// deployment lifecycle
	for _, tc := range []struct {
		CaseName string
		Release  string
		Values   map[string]string

		ExpectedLifecycle *coreV1.Lifecycle
	}{
		{
			CaseName: "lifecycle",
			Release:  "production",
			Values: map[string]string{
				"lifecycle.preStop.exec.command[0]": "/bin/sh",
				"lifecycle.preStop.exec.command[1]": "-c",
				"lifecycle.preStop.exec.command[2]": "sleep 10",
			},
			ExpectedLifecycle: &coreV1.Lifecycle{
				PreStop: &coreV1.LifecycleHandler{
					Exec: &coreV1.ExecAction{
						Command: []string{"/bin/sh", "-c", "sleep 10"},
					},
				},
			},
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedLifecycle, deployment.Spec.Template.Spec.Containers[0].Lifecycle)
		})
	}

	// deployment livenessProbe, readinessProbe, and startupProbe tests
	for _, tc := range []struct {
		CaseName string
		Release  string
		Values   map[string]string

		ExpectedLivenessProbe  *coreV1.Probe
		ExpectedReadinessProbe *coreV1.Probe
		ExpectedStartupProbe   *coreV1.Probe
	}{
		{
			CaseName:               "defaults",
			Release:                "production",
			ExpectedLivenessProbe:  defaultLivenessProbe(),
			ExpectedReadinessProbe: defaultReadinessProbe(),
			ExpectedStartupProbe:   nil,
		},
		{
			CaseName: "custom liveness probe",
			Release:  "production",
			Values: map[string]string{
				"livenessProbe.port":                 "1234",
				"livenessProbe.httpHeaders[0].name":  "custom-header",
				"livenessProbe.httpHeaders[0].value": "awesome",
			},
			ExpectedLivenessProbe: &coreV1.Probe{
				ProbeHandler: coreV1.ProbeHandler{
					HTTPGet: &coreV1.HTTPGetAction{
						Path:   "/",
						Port:   intstr.FromInt(1234),
						Scheme: coreV1.URISchemeHTTP,
						HTTPHeaders: []coreV1.HTTPHeader{
							{
								Name:  "custom-header",
								Value: "awesome",
							},
						},
					},
				},
				InitialDelaySeconds: 15,
				TimeoutSeconds:      15,
			},
			ExpectedReadinessProbe: defaultReadinessProbe(),
			ExpectedStartupProbe:   nil,
		},
		{
			CaseName: "exec liveness probe",
			Release:  "production",
			Values: map[string]string{
				"livenessProbe.command[0]": "echo",
				"livenessProbe.command[1]": "hello",
				"livenessProbe.probeType":  "exec",
			},
			ExpectedLivenessProbe: &coreV1.Probe{
				ProbeHandler: coreV1.ProbeHandler{
					Exec: &coreV1.ExecAction{
						Command: []string{"echo", "hello"},
					},
				},
				InitialDelaySeconds: 15,
				TimeoutSeconds:      15,
			},
			ExpectedReadinessProbe: defaultReadinessProbe(),
			ExpectedStartupProbe:   nil,
		},
		{
			CaseName: "custom readiness probe",
			Release:  "production",
			Values: map[string]string{
				"readinessProbe.port":                 "2345",
				"readinessProbe.httpHeaders[0].name":  "custom-header",
				"readinessProbe.httpHeaders[0].value": "awesome",
			},
			ExpectedLivenessProbe: defaultLivenessProbe(),
			ExpectedReadinessProbe: &coreV1.Probe{
				ProbeHandler: coreV1.ProbeHandler{
					HTTPGet: &coreV1.HTTPGetAction{
						Path:   "/",
						Port:   intstr.FromInt(2345),
						Scheme: coreV1.URISchemeHTTP,
						HTTPHeaders: []coreV1.HTTPHeader{
							{
								Name:  "custom-header",
								Value: "awesome",
							},
						},
					},
				},
				InitialDelaySeconds: 5,
				TimeoutSeconds:      3,
			},
			ExpectedStartupProbe: nil,
		},
		{
			CaseName: "exec readiness probe",
			Release:  "production",
			Values: map[string]string{
				"readinessProbe.command[0]": "echo",
				"readinessProbe.command[1]": "hello",
				"readinessProbe.probeType":  "exec",
			},
			ExpectedLivenessProbe: defaultLivenessProbe(),
			ExpectedReadinessProbe: &coreV1.Probe{
				ProbeHandler: coreV1.ProbeHandler{
					Exec: &coreV1.ExecAction{
						Command: []string{"echo", "hello"},
					},
				},
				InitialDelaySeconds: 5,
				TimeoutSeconds:      3,
			},
			ExpectedStartupProbe: nil,
		},
		{
			CaseName: "custom startup probe",
			Release:  "production",
			Values: map[string]string{
				"startupProbe.enabled":              "true",
				"startupProbe.port":                 "2345",
				"startupProbe.httpHeaders[0].name":  "custom-header",
				"startupProbe.httpHeaders[0].value": "awesome",
			},
			ExpectedLivenessProbe:  defaultLivenessProbe(),
			ExpectedReadinessProbe: defaultReadinessProbe(),
			ExpectedStartupProbe: &coreV1.Probe{
				ProbeHandler: coreV1.ProbeHandler{
					HTTPGet: &coreV1.HTTPGetAction{
						Path:   "/",
						Port:   intstr.FromInt(2345),
						Scheme: coreV1.URISchemeHTTP,
						HTTPHeaders: []coreV1.HTTPHeader{
							{
								Name:  "custom-header",
								Value: "awesome",
							},
						},
					},
				},
				InitialDelaySeconds: 5,
				TimeoutSeconds:      3,
				FailureThreshold:    30,
				PeriodSeconds:       10,
			},
		},
		{
			CaseName: "exec startup probe",
			Release:  "production",
			Values: map[string]string{
				"startupProbe.enabled":    "true",
				"startupProbe.command[0]": "echo",
				"startupProbe.command[1]": "hello",
				"startupProbe.probeType":  "exec",
			},
			ExpectedLivenessProbe:  defaultLivenessProbe(),
			ExpectedReadinessProbe: defaultReadinessProbe(),
			ExpectedStartupProbe: &coreV1.Probe{
				ProbeHandler: coreV1.ProbeHandler{
					Exec: &coreV1.ExecAction{
						Command: []string{"echo", "hello"},
					},
				},
				InitialDelaySeconds: 5,
				TimeoutSeconds:      3,
				FailureThreshold:    30,
				PeriodSeconds:       10,
			},
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedLivenessProbe, deployment.Spec.Template.Spec.Containers[0].LivenessProbe)
			require.Equal(t, tc.ExpectedReadinessProbe, deployment.Spec.Template.Spec.Containers[0].ReadinessProbe)
			require.Equal(t, tc.ExpectedStartupProbe, deployment.Spec.Template.Spec.Containers[0].StartupProbe)
		})
	}

	// deployment hostAliases
	for _, tc := range []struct {
		CaseName string
		Release  string
		Values   map[string]string

		ExpectedHostAliases []coreV1.HostAlias
	}{
		{
			CaseName:            "default hostAliases",
			Release:             "production",
			ExpectedHostAliases: nil,
		},
		{
			CaseName: "hostAliases for two IP addresses",
			Release:  "production",
			Values: map[string]string{
				"hostAliases[0].ip":           "1.2.3.4",
				"hostAliases[0].hostnames[0]": "host1.example1.com",
				"hostAliases[1].ip":           "5.6.7.8",
				"hostAliases[1].hostnames[0]": "host1.example2.com",
				"hostAliases[1].hostnames[1]": "host2.example2.com",
			},

			ExpectedHostAliases: []coreV1.HostAlias{
				{
					IP:        "1.2.3.4",
					Hostnames: []string{"host1.example1.com"},
				},
				{
					IP:        "5.6.7.8",
					Hostnames: []string{"host1.example2.com", "host2.example2.com"},
				},
			},
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedHostAliases, deployment.Spec.Template.Spec.HostAliases)
		})
	}

	// deployment dnsConfig
	for _, tc := range []struct {
		CaseName string
		Release  string
		Values   map[string]string

		ExpectedDnsConfig *coreV1.PodDNSConfig
	}{
		{
			CaseName:          "default dnsConfig",
			Release:           "production",
			ExpectedDnsConfig: nil,
		},
		{
			CaseName: "dnsConfig with different DNS",
			Release:  "production",
			Values: map[string]string{
				"dnsConfig.nameservers[0]":  "1.2.3.4",
				"dnsConfig.options[0].name": "edns0",
			},

			ExpectedDnsConfig: &coreV1.PodDNSConfig{
				Nameservers: []string{"1.2.3.4"},
				Options: []coreV1.PodDNSConfigOption{
					{
						Name: "edns0",
					},
				},
			},
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedDnsConfig, deployment.Spec.Template.Spec.DNSConfig)
		})
	}

	// resources
	for _, tc := range []struct {
		CaseName string
		Values   map[string]string
		Release  string

		EoxpectedNodeSelector map[string]string
		ExpectedResources     coreV1.ResourceRequirements
	}{
		{
			CaseName: "default",
			Release:  "production",
			Values:   map[string]string{},

			ExpectedResources: coreV1.ResourceRequirements{
				Limits:   coreV1.ResourceList(nil),
				Requests: coreV1.ResourceList{},
			},
		},
		{
			CaseName: "added resources",
			Release:  "production",
			Values: map[string]string{
				"resources.limits.cpu":      "500m",
				"resources.limits.memory":   "4Gi",
				"resources.requests.cpu":    "200m",
				"resources.requests.memory": "2Gi",
			},

			ExpectedResources: coreV1.ResourceRequirements{
				Limits: coreV1.ResourceList{
					"cpu":    resource.MustParse("500m"),
					"memory": resource.MustParse("4Gi")},
				Requests: coreV1.ResourceList{
					"cpu":    resource.MustParse("200m"),
					"memory": resource.MustParse("2Gi"),
				},
			},
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedResources, deployment.Spec.Template.Spec.Containers[0].Resources)
		})
	}

	// Test Deployment selector
	for _, tc := range []struct {
		CaseName string
		Release  string
		Values   map[string]string

		ExpectedName                      string
		ExpectedRelease                   string
		ExpectedSelector                  *metav1.LabelSelector
		ExpectedNodeSelector              map[string]string
		ExpectedTolerations               []coreV1.Toleration
		ExpectedInitContainers            []coreV1.Container
		ExpectedAffinity                  *coreV1.Affinity
		ExpectedTopologySpreadConstraints []coreV1.TopologySpreadConstraint
	}{
		{
			CaseName:        "selector",
			Release:         "production",
			ExpectedName:    "production",
			ExpectedRelease: "production",
			ExpectedSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     "production",
					"release": "production",
					"tier":    "web",
					"track":   "stable",
				},
			},
		},
		{
			CaseName: "nodeSelector",
			Release:  "production",
			Values: map[string]string{
				"nodeSelector.disktype": "ssd",
			},
			ExpectedName:    "production",
			ExpectedRelease: "production",
			ExpectedSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     "production",
					"release": "production",
					"tier":    "web",
					"track":   "stable",
				},
			},
			ExpectedNodeSelector: map[string]string{
				"disktype": "ssd",
			},
		},
		{
			CaseName: "tolerations",
			Release:  "production",
			Values: map[string]string{
				"tolerations[0].key":      "key1",
				"tolerations[0].operator": "Equal",
				"tolerations[0].value":    "value1",
				"tolerations[0].effect":   "NoSchedule",
			},
			ExpectedName:    "production",
			ExpectedRelease: "production",
			ExpectedSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     "production",
					"release": "production",
					"tier":    "web",
					"track":   "stable",
				},
			},
			ExpectedTolerations: []coreV1.Toleration{
				{
					Key:      "key1",
					Operator: "Equal",
					Value:    "value1",
					Effect:   "NoSchedule",
				},
			},
		},
		{
			CaseName: "initContainers",
			Release:  "production",
			Values: map[string]string{
				"initContainers[0].name":       "myservice",
				"initContainers[0].image":      "myimage:1",
				"initContainers[0].command[0]": "sh",
				"initContainers[0].command[1]": "-c",
				"initContainers[0].command[2]": "until nslookup myservice; do echo waiting for myservice to start; sleep 1; done;",
			},

			ExpectedName:    "production",
			ExpectedRelease: "production",
			ExpectedSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     "production",
					"release": "production",
					"tier":    "web",
					"track":   "stable",
				},
			},
			ExpectedInitContainers: []coreV1.Container{
				{
					Name:    "myservice",
					Image:   "myimage:1",
					Command: []string{"sh", "-c", "until nslookup myservice; do echo waiting for myservice to start; sleep 1; done;"},
				},
			},
		},
		{
			CaseName: "affinity",
			Release:  "production",
			Values: map[string]string{
				"affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key":      "key1",
				"affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].operator": "DoesNotExist",
			},
			ExpectedName:    "production",
			ExpectedRelease: "production",
			ExpectedSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     "production",
					"release": "production",
					"tier":    "web",
					"track":   "stable",
				},
			},
			ExpectedAffinity: &coreV1.Affinity{
				NodeAffinity: &coreV1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &coreV1.NodeSelector{
						NodeSelectorTerms: []coreV1.NodeSelectorTerm{
							{
								MatchExpressions: []coreV1.NodeSelectorRequirement{
									{
										Key:      "key1",
										Operator: "DoesNotExist",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			CaseName: "topologySpreadConstraints",
			Release:  "production",
			Values: map[string]string{
				"topologySpreadConstraints[0].maxSkew":                                     "1",
				"topologySpreadConstraints[0].topologyKey":                                 "zone",
				"topologySpreadConstraints[0].whenUnsatisfiable":                           "DoNotSchedule",
				"topologySpreadConstraints[0].labelSelector.matchLabels.foo":               "bar",
				"topologySpreadConstraints[0].labelSelector.matchExpressions[0].key":       "key1",
				"topologySpreadConstraints[0].labelSelector.matchExpressions[0].operator":  "DoesNotExist",
				"topologySpreadConstraints[0].labelSelector.matchExpressions[0].values[0]": "value1",
			},
			ExpectedName:    "production",
			ExpectedRelease: "production",
			ExpectedSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     "production",
					"release": "production",
					"tier":    "web",
					"track":   "stable",
				},
			},
			ExpectedTopologySpreadConstraints: []coreV1.TopologySpreadConstraint{
				{
					MaxSkew:           1,
					TopologyKey:       "zone",
					WhenUnsatisfiable: "DoNotSchedule",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "key1",
								Operator: "DoesNotExist",
								Values:   []string{"value1"},
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, tc.Release, []string{"templates/deployment.yaml"}, nil)

			var deployment appsV1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, tc.ExpectedName, deployment.Name)
			require.Equal(t, map[string]string{
				"app":                          tc.ExpectedName,
				"chart":                        chartName,
				"heritage":                     "Helm",
				"release":                      tc.ExpectedRelease,
				"tier":                         "web",
				"track":                        "stable",
				"app.kubernetes.io/name":       tc.ExpectedName,
				"helm.sh/chart":                chartName,
				"app.kubernetes.io/managed-by": "Helm",
				"app.kubernetes.io/instance":   tc.ExpectedRelease,
			}, deployment.Labels)

			require.Equal(t, tc.ExpectedSelector, deployment.Spec.Selector)

			require.Equal(t, map[string]string{
				"app":                          tc.ExpectedName,
				"chart":                        chartName,
				"heritage":                     "Helm",
				"release":                      tc.ExpectedRelease,
				"tier":                         "web",
				"track":                        "stable",
				"app.kubernetes.io/name":       tc.ExpectedName,
				"helm.sh/chart":                chartName,
				"app.kubernetes.io/managed-by": "Helm",
				"app.kubernetes.io/instance":   tc.ExpectedRelease,
			}, deployment.Spec.Template.Labels)

			require.Equal(t, tc.ExpectedNodeSelector, deployment.Spec.Template.Spec.NodeSelector)
			require.Equal(t, tc.ExpectedTolerations, deployment.Spec.Template.Spec.Tolerations)
			require.Equal(t, tc.ExpectedInitContainers, deployment.Spec.Template.Spec.InitContainers)
			require.Equal(t, tc.ExpectedAffinity, deployment.Spec.Template.Spec.Affinity)
			require.Equal(t, tc.ExpectedTopologySpreadConstraints, deployment.Spec.Template.Spec.TopologySpreadConstraints)
		})
	}
}

func TestServiceExtraPortServicePortDefinition(t *testing.T) {
	releaseName := "deployment-extra-ports-service-port-definition-test"
	templates := []string{"templates/deployment.yaml"}

	tcs := []struct {
		name                string
		values              map[string]string
		valueFiles          []string
		expectedPorts       []coreV1.ContainerPort
		expectedErrorRegexp *regexp.Regexp
	}{
		{
			name:       "with extra ports service port",
			valueFiles: []string{"../testdata/service-definition.yaml"},
			expectedPorts: []coreV1.ContainerPort{
				{
					Name:          "web",
					ContainerPort: 5000,
				},
				{
					Name:          "port-443",
					ContainerPort: 443,
					Protocol:      "TCP",
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			opts := &helm.Options{
				ValuesFiles: tc.valueFiles,
				SetValues:   tc.values,
			}
			output := mustRenderTemplate(t, opts, releaseName, templates, nil)

			deployment := new(appsV1.Deployment)
			helm.UnmarshalK8SYaml(t, output, deployment)
			require.Equal(t, tc.expectedPorts, deployment.Spec.Template.Spec.Containers[0].Ports)
		})
	}
}

func TestDeploymentTemplateWithVolumeMounts(t *testing.T) {
	releaseName := "deployment-with-volume-mounts-test"
	templates := []string{"templates/deployment.yaml"}

	hostPathDirectoryType := coreV1.HostPathDirectory
	configMapOptional := false
	configMapDefaultMode := coreV1.ConfigMapVolumeSourceDefaultMode

	tcs := []struct {
		name                 string
		values               map[string]string
		valueFiles           []string
		expectedVolumes      []coreV1.Volume
		expectedVolumeMounts []coreV1.VolumeMount
		expectedErrorRegexp  *regexp.Regexp
	}{
		{
			name:       "with volume mounts",
			valueFiles: []string{"../testdata/volume-mounts.yaml"},
			expectedVolumes: []coreV1.Volume{
				{
					Name: "log-dir",
					VolumeSource: coreV1.VolumeSource{
						PersistentVolumeClaim: &coreV1.PersistentVolumeClaimVolumeSource{
							ClaimName: "deployment-with-volume-mounts-test-auto-deploy-log-dir",
						},
					},
				},
				{
					Name: "config",
					VolumeSource: coreV1.VolumeSource{
						PersistentVolumeClaim: &coreV1.PersistentVolumeClaimVolumeSource{
							ClaimName: "deployment-with-volume-mounts-test-auto-deploy-config",
						},
					},
				},
			},
			expectedVolumeMounts: []coreV1.VolumeMount{
				{
					Name:      "log-dir",
					MountPath: "/log",
				},
				{
					Name:      "config",
					MountPath: "/app-config",
					SubPath:   "config.txt",
				},
			},
		},
		{
			name:       "with extra volume mounts",
			valueFiles: []string{"../testdata/extra-volume-mounts.yaml"},
			expectedVolumes: []coreV1.Volume{
				{
					Name: "config-volume",
					VolumeSource: coreV1.VolumeSource{
						ConfigMap: &coreV1.ConfigMapVolumeSource{
							coreV1.LocalObjectReference{
								Name: "test-config",
							},
							[]coreV1.KeyToPath{},
							&configMapDefaultMode,
							&configMapOptional,
						},
					},
				},
				{
					Name: "test-host-path",
					VolumeSource: coreV1.VolumeSource{
						HostPath: &coreV1.HostPathVolumeSource{
							Path: "/etc/ssl/certs/",
							Type: &hostPathDirectoryType,
						},
					},
				},
				{
					Name: "secret-volume",
					VolumeSource: coreV1.VolumeSource{
						Secret: &coreV1.SecretVolumeSource{
							SecretName: "mysecret",
						},
					},
				},
			},
			expectedVolumeMounts: []coreV1.VolumeMount{
				{
					Name:      "config-volume",
					MountPath: "/app/config.yaml",
					SubPath:   "config.yaml",
				},
				{
					Name:      "test-host-path",
					MountPath: "/etc/ssl/certs/",
					ReadOnly:  true,
				},
				{
					Name:      "secret-volume",
					MountPath: "/etc/specialSecret",
					ReadOnly:  true,
				},
			},
		},
		{
			name:       "with extra volume mounts and persistence",
			valueFiles: []string{"../testdata/mix-volume-mounts.yaml"},
			expectedVolumes: []coreV1.Volume{
				{
					Name: "log-dir",
					VolumeSource: coreV1.VolumeSource{
						PersistentVolumeClaim: &coreV1.PersistentVolumeClaimVolumeSource{
							ClaimName: "deployment-with-volume-mounts-test-auto-deploy-log-dir",
						},
					},
				},
				{
					Name: "config-volume",
					VolumeSource: coreV1.VolumeSource{
						ConfigMap: &coreV1.ConfigMapVolumeSource{
							coreV1.LocalObjectReference{
								Name: "test-config",
							},
							[]coreV1.KeyToPath{},
							&configMapDefaultMode,
							&configMapOptional,
						},
					},
				},
			},
			expectedVolumeMounts: []coreV1.VolumeMount{
				{
					Name:      "log-dir",
					MountPath: "/log",
				},
				{
					Name:      "config-volume",
					MountPath: "/app/config.yaml",
					SubPath:   "config.yaml",
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			opts := &helm.Options{
				ValuesFiles: tc.valueFiles,
				SetValues:   tc.values,
			}
			output := mustRenderTemplate(t, opts, releaseName, templates, nil)

			deployment := new(appsV1.Deployment)
			helm.UnmarshalK8SYaml(t, output, deployment)

			for i, expectedVolume := range tc.expectedVolumes {
				require.Equal(t, expectedVolume.Name, deployment.Spec.Template.Spec.Volumes[i].Name)
				if deployment.Spec.Template.Spec.Volumes[i].PersistentVolumeClaim != nil {
					require.Equal(t, expectedVolume.PersistentVolumeClaim.ClaimName, deployment.Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName)
				}
				if deployment.Spec.Template.Spec.Volumes[i].ConfigMap != nil {
					require.Equal(t, expectedVolume.ConfigMap.Name, deployment.Spec.Template.Spec.Volumes[i].ConfigMap.Name)
				}
				if deployment.Spec.Template.Spec.Volumes[i].HostPath != nil {
					require.Equal(t, expectedVolume.HostPath.Path, deployment.Spec.Template.Spec.Volumes[i].HostPath.Path)
					require.Equal(t, expectedVolume.HostPath.Type, deployment.Spec.Template.Spec.Volumes[i].HostPath.Type)
				}
				if deployment.Spec.Template.Spec.Volumes[i].Secret != nil {
					require.Equal(t, expectedVolume.Secret.SecretName, deployment.Spec.Template.Spec.Volumes[i].Secret.SecretName)
				}
			}

			for i, expectedVolumeMount := range tc.expectedVolumeMounts {
				require.Equal(t, expectedVolumeMount.Name, deployment.Spec.Template.Spec.Containers[0].VolumeMounts[i].Name)
				require.Equal(t, expectedVolumeMount.MountPath, deployment.Spec.Template.Spec.Containers[0].VolumeMounts[i].MountPath)
				require.Equal(t, expectedVolumeMount.SubPath, deployment.Spec.Template.Spec.Containers[0].VolumeMounts[i].SubPath)
			}
		})
	}
}

func TestDeploymentDatabaseUrlEnvironmentVariable(t *testing.T) {
	releaseName := "deployment-application-database-url-test"

	tcs := []struct {
		CaseName            string
		Values              map[string]string
		ExpectedDatabaseUrl string
		Template            string
	}{
		{
			CaseName: "present-deployment",
			Values: map[string]string{
				"application.database_url": "PRESENT",
			},
			ExpectedDatabaseUrl: "PRESENT",
			Template:            "templates/deployment.yaml",
		},
		{
			CaseName: "missing-deployment",
			Template: "templates/deployment.yaml",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.CaseName, func(t *testing.T) {

			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{
				"gitlab.app": "auto-devops-examples/minimal-ruby-app",
				"gitlab.env": "prod",
			}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, releaseName, []string{tc.Template}, nil)

			deployment := new(appsV1.Deployment)
			helm.UnmarshalK8SYaml(t, output, &deployment)

			if tc.ExpectedDatabaseUrl != "" {
				require.Contains(t, deployment.Spec.Template.Spec.Containers[0].Env, coreV1.EnvVar{Name: "DATABASE_URL", Value: tc.ExpectedDatabaseUrl})
			} else {
				for _, envVar := range deployment.Spec.Template.Spec.Containers[0].Env {
					require.NotEqual(t, "DATABASE_URL", envVar.Name)
				}
			}
		})
	}
}

func TestDeploymentTemplateWithExtraEnvFrom(t *testing.T) {
	releaseName := "deployment-with-extra-envfrom-test"
	templates := []string{"templates/deployment.yaml"}

	tcs := []struct {
		name            string
		values          map[string]string
		expectedEnvFrom coreV1.EnvFromSource
	}{
		{
			name: "with extra envfrom secret test",
			values: map[string]string{
				"extraEnvFrom[0].secretRef.name": "secret-name-test",
			},
			expectedEnvFrom: coreV1.EnvFromSource{
				SecretRef: &coreV1.SecretEnvSource{
					LocalObjectReference: coreV1.LocalObjectReference{
						Name: "secret-name-test",
					},
				},
			},
		},
		{
			name: "with extra envfrom with secretName test",
			values: map[string]string{
				"application.secretName":         "gitlab-secretname-test",
				"extraEnvFrom[0].secretRef.name": "secret-name-test",
			},
			expectedEnvFrom: coreV1.EnvFromSource{
				SecretRef: &coreV1.SecretEnvSource{
					LocalObjectReference: coreV1.LocalObjectReference{
						Name: "secret-name-test",
					},
				},
			},
		},
		{
			name: "with extra envfrom configmap test",
			values: map[string]string{
				"extraEnvFrom[0].configMapRef.name": "configmap-name-test",
			},
			expectedEnvFrom: coreV1.EnvFromSource{
				ConfigMapRef: &coreV1.ConfigMapEnvSource{
					LocalObjectReference: coreV1.LocalObjectReference{
						Name: "configmap-name-test",
					},
				},
			},
		},
		{
			name: "test with extra env from secret using templating values",
			values: map[string]string{
				"extraEnvFrom[0].secretRef.name": "secret-name-{{ .Release.Name }}",
			},
			expectedEnvFrom: coreV1.EnvFromSource{
				SecretRef: &coreV1.SecretEnvSource{
					LocalObjectReference: coreV1.LocalObjectReference{
						Name: "secret-name-" + releaseName,
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			namespaceName := "test-namespace-" + strings.ToLower(random.UniqueId())
			opts := &helm.Options{
				SetValues:      tc.values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}
			output := mustRenderTemplate(t, opts, releaseName, templates, nil)

			deployment := new(appsV1.Deployment)
			helm.UnmarshalK8SYaml(t, output, deployment)
			require.Contains(t, deployment.Spec.Template.Spec.Containers[0].EnvFrom, tc.expectedEnvFrom)
		})
	}
}

func TestDeploymentTemplateWithExtraEnv(t *testing.T) {
	releaseName := "deployment-with-extra-env-test"
	templates := []string{"templates/deployment.yaml"}

	tcs := []struct {
		name        string
		values      map[string]string
		expectedEnv coreV1.EnvVar
	}{
		{
			name: "with extra env secret test",
			values: map[string]string{
				"extraEnv[0].name":  "env-name-test",
				"extraEnv[0].value": "test-value",
			},
			expectedEnv: coreV1.EnvVar{
				Name:  "env-name-test",
				Value: "test-value",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			opts := &helm.Options{
				SetValues: tc.values,
			}
			output := mustRenderTemplate(t, opts, releaseName, templates, nil)

			deployment := new(appsV1.Deployment)
			helm.UnmarshalK8SYaml(t, output, deployment)
			require.Contains(t, deployment.Spec.Template.Spec.Containers[0].Env, tc.expectedEnv)
		})
	}
}

func TestDeploymentTemplateWithSecurityContext(t *testing.T) {
	releaseName := "deployment-with-security-context"
	templates := []string{"templates/deployment.yaml"}

	tcs := []struct {
		name                        string
		values                      map[string]string
		expectedSecurityContextName string
	}{
		{
			name: "with gMSA security context",
			values: map[string]string{
				"securityContext.windowsOptions.gmsaCredentialSpecName": "gmsa-test",
			},
			expectedSecurityContextName: "gmsa-test",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			opts := &helm.Options{
				SetValues: tc.values,
			}
			output := mustRenderTemplate(t, opts, releaseName, templates, nil)

			deployment := new(appsV1.Deployment)

			helm.UnmarshalK8SYaml(t, output, deployment)
			require.Equal(t, *deployment.Spec.Template.Spec.SecurityContext.WindowsOptions.GMSACredentialSpecName, tc.expectedSecurityContextName)
		})
	}
}

func TestDeploymentTemplateWithContainerSecurityContext(t *testing.T) {
	releaseName := "deployment-with-container-security-context"
	templates := []string{"templates/deployment.yaml"}

	tcs := []struct {
		name                                string
		values                              map[string]string
		expectedSecurityContextCapabilities []coreV1.Capability
	}{
		{
			name: "with container security context capabilities",
			values: map[string]string{
				"containerSecurityContext.capabilities.drop[0]": "ALL",
			},
			expectedSecurityContextCapabilities: []coreV1.Capability{
				"ALL",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			opts := &helm.Options{
				SetValues: tc.values,
			}
			output := mustRenderTemplate(t, opts, releaseName, templates, nil)

			deployment := new(appsV1.Deployment)

			helm.UnmarshalK8SYaml(t, output, deployment)
			require.Equal(t, deployment.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Drop, tc.expectedSecurityContextCapabilities)
		})
	}
}
