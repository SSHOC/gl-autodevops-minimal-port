package main

import (
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
)

func TestInitializeDatabaseUrlEnvironmentVariable(t *testing.T) {
	releaseName := "initialize-application-database-url-test"

	tcs := []struct {
		CaseName            string
		Values              map[string]string
		ExpectedDatabaseUrl string
		Template            string
	}{
		{
			CaseName: "present-db-intialize",
			Values: map[string]string{
				"application.database_url":      "PRESENT",
				"application.initializeCommand": "echo initialize",
			},
			ExpectedDatabaseUrl: "PRESENT",
			Template:            "templates/db-initialize-job.yaml",
		},
		{
			CaseName: "missing-db-initialize",
			Values: map[string]string{
				"application.initializeCommand": "echo initialize",
			},
			Template: "templates/db-initialize-job.yaml",
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

func TestInitializeDatabaseImagePullSecrets(t *testing.T) {
	releaseName := "initialize-application-database-image-pull-secrets"

	tcs := []struct {
		CaseName                 string
		Values                   map[string]string
		ExpectedImagePullSecrets []coreV1.LocalObjectReference
		Template                 string
	}{
		{
			CaseName: "default-secret",
			Values: map[string]string{
				"application.initializeCommand": "echo initialize",
			},
			ExpectedImagePullSecrets: []coreV1.LocalObjectReference{
				{
					Name: "gitlab-registry",
				},
			},
			Template: "templates/db-initialize-job.yaml",
		},
		{
			CaseName: "present-secret",
			Values: map[string]string{
				"application.initializeCommand": "echo initialize",
				"image.secrets[0].name":         "expected-secret",
			},
			ExpectedImagePullSecrets: []coreV1.LocalObjectReference{
				{
					Name: "expected-secret",
				},
			},
			Template: "templates/db-initialize-job.yaml",
		},
		{
			CaseName: "multiple-secrets",
			Values: map[string]string{
				"application.initializeCommand": "echo initialize",
				"image.secrets[0].name":         "expected-secret",
				"image.secrets[1].name":         "additional-secret",
			},
			ExpectedImagePullSecrets: []coreV1.LocalObjectReference{
				{
					Name: "expected-secret",
				},
				{
					Name: "additional-secret",
				},
			},
			Template: "templates/db-initialize-job.yaml",
		},
		{
			CaseName: "missing-secret",
			Values: map[string]string{
				"application.initializeCommand": "echo initialize",
				"image.secrets":                 "null",
			},
			ExpectedImagePullSecrets: nil,
			Template:                 "templates/db-initialize-job.yaml",
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

			require.Equal(t, tc.ExpectedImagePullSecrets, deployment.Spec.Template.Spec.ImagePullSecrets)
		})
	}
}

func TestInitializeDatabaseLabels(t *testing.T) {
	releaseName := "initialize-application-database-labels"

	for _, tc := range []struct {
		CaseName       string
		Values         map[string]string
		Release        string
		ExpectedLabels map[string]string
		Template       string
	}{
		{
			CaseName: "no label",
			Release:  "production",
			Values: map[string]string{
				"application.initializeCommand": "echo initialize",
			},
			ExpectedLabels: nil,
			Template:       "templates/db-initialize-job.yaml",
		},
		{
			CaseName: "one label",
			Release:  "production",
			Values: map[string]string{
				"application.initializeCommand": "echo initialize",
				"extraLabels.firstLabel":        "expected-label",
			},
			ExpectedLabels: map[string]string{
				"firstLabel": "expected-label",
			},
			Template: "templates/db-initialize-job.yaml",
		},
		{
			CaseName: "multiple labels",
			Release:  "production",
			Values: map[string]string{
				"application.initializeCommand": "echo initialize",
				"extraLabels.firstLabel":        "expected-label",
				"extraLabels.secondLabel":       "expected-label",
			},
			ExpectedLabels: map[string]string{
				"firstLabel":  "expected-label",
				"secondLabel": "expected-label",
			},
			Template: "templates/db-initialize-job.yaml",
		},
	} {
		t.Run(tc.CaseName, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			values := map[string]string{}

			mergeStringMap(values, tc.Values)

			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, releaseName, []string{tc.Template}, nil)

			deployment := new(appsV1.Deployment)
			helm.UnmarshalK8SYaml(t, output, &deployment)

			for key, value := range tc.ExpectedLabels {
				require.Equal(t, deployment.ObjectMeta.Labels[key], value)
				require.Equal(t, deployment.Spec.Template.ObjectMeta.Labels[key], value)
			}
		})
	}
}

func TestInitializeDatabaseTemplateWithExtraEnvFrom(t *testing.T) {
	releaseName := "initialize-application-database-extra-envfrom"
	templates := []string{"templates/db-initialize-job.yaml"}

	tcs := []struct {
		name            string
		values          map[string]string
		expectedEnvFrom coreV1.EnvFromSource
	}{
		{
			name: "with extra envfrom secret test",
			values: map[string]string{
				"application.initializeCommand":  "echo initialize",
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
			name: "test with extra env from secret using templating values",
			values: map[string]string{
				"application.initializeCommand":  "echo initialize",
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
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			options := &helm.Options{
				SetValues:      tc.values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, releaseName, templates, nil)

			var deployments deploymentAppsV1List
			helm.UnmarshalK8SYaml(t, output, &deployments)
			for _, deployment := range deployments.Items {
				require.Contains(t, deployment.Spec.Template.Spec.Containers[0].EnvFrom, tc.expectedEnvFrom)
			}
		})
	}
}

func TestInitializeDatabaseTemplateWithExtraEnv(t *testing.T) {
	releaseName := "initialize-application-database-extra-env"
	templates := []string{"templates/db-initialize-job.yaml"}

	tcs := []struct {
		name        string
		values      map[string]string
		expectedEnv coreV1.EnvVar
	}{
		{
			name: "with extra env secret test",
			values: map[string]string{
				"application.initializeCommand": "echo initialize",
				"extraEnv[0].name":              "env-name-test",
				"extraEnv[0].value":             "test-value",
			},
			expectedEnv: coreV1.EnvVar{
				Name:  "env-name-test",
				Value: "test-value",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			namespaceName := "minimal-ruby-app-" + strings.ToLower(random.UniqueId())

			options := &helm.Options{
				SetValues:      tc.values,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
			}

			output := mustRenderTemplate(t, options, releaseName, templates, nil)

			var deployments deploymentAppsV1List
			helm.UnmarshalK8SYaml(t, output, &deployments)
			for _, deployment := range deployments.Items {
				require.Contains(t, deployment.Spec.Template.Spec.Containers[0].Env, tc.expectedEnv)
			}
		})
	}
}
