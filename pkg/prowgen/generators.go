/*
Copyright 2022 The cert-manager Authors.

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

package prowgen

import (
	"fmt"
	"strings"
)

// MakeTest generates a test which runs linting and verification targets as well as
// unit and integration tests
func MakeTest() *Job {
	job := jobTemplate(
		"make-test",
		"Runs unit and integration tests and verification scripts",
		addServiceAccountLabel,
		addMakeVolumesLabel,
		addMaxConcurrency(8),
	)

	makeJobs, cpuRequest := calculateMakeConcurrency("2000m")

	job.Spec.Containers = []Container{
		{
			Image: CommonTestImage,
			Args: []string{
				"runner",
				"make",
				makeJobs,
				"vendor-go",
				"ci-presubmit",
				"test-ci",
			},
			Resources: ContainerResources{
				Requests: ContainerResourceRequest{
					CPU:    cpuRequest,
					Memory: "4Gi",
				},
			},
		},
	}

	return job
}

// ChartTest generates a test which lints helm charts. This is run inside a container
// and so requires additional permissions.
func ChartTest() *Job {
	job := jobTemplate(
		"chart",
		"Verifies the Helm chart passes linting checks",
		addServiceAccountLabel,
		addDindLabel,
		addMakeVolumesLabel,
		addMaxConcurrency(8),
	)

	job.Spec.Containers = []Container{
		{
			Image: CommonTestImage,
			Args: []string{
				"runner",
				"make",
				"vendor-go",
				"verify-chart",
			},
			Resources: ContainerResources{
				Requests: ContainerResourceRequest{
					CPU:    "1",
					Memory: "1Gi",
				},
			},
			SecurityContext: &SecurityContext{
				Privileged: true,
			},
		},
	}

	return job
}

// E2ETest generates a test which runs end-to-end tests with feature gates enabled. This
// is run inside a container and requires additional permissions.
func E2ETest(k8sVersion string) *Job {
	// we don't want to use dots in names, so replace with dashes
	nameVersion := strings.ReplaceAll(k8sVersion, ".", "-")

	desc := fmt.Sprintf("Runs the end-to-end test suite against a Kubernetes v%s cluster", k8sVersion)

	job := jobTemplate(
		"e2e-v"+nameVersion,
		desc,
		addServiceAccountLabel,
		addDindLabel,
		addCloudflareCredentialsLabel,
		addMakeVolumesLabel,
		addStandardE2ELabels(k8sVersion),
		addRetryFlakesLabel,
		addMaxConcurrency(4),
	)

	makeJobs, cpuRequest := calculateMakeConcurrency("3500m")

	k8sVersionArg := fmt.Sprintf("K8S_VERSION=%s", k8sVersion)

	job.Spec.Containers = []Container{
		{
			Image: CommonTestImage,
			Args: []string{
				"runner",
				"make",
				makeJobs,
				"vendor-go",
				"e2e-ci",
				k8sVersionArg,
			},
			Resources: ContainerResources{
				Requests: ContainerResourceRequest{
					CPU:    cpuRequest,
					Memory: "12Gi",
				},
			},
			SecurityContext: &SecurityContext{
				Privileged: true,
				Capabilities: &SecurityContextCapabilities{
					Add: []string{"SYS_ADMIN"},
				},
			},
		},
	}

	return job
}

// E2ETestVenafiTPP generates a test which runs end-to-end tests only focusing on Venafi TPP.
// This runs inside a container and so requires additional permissions.
func E2ETestVenafiTPP(k8sVersion string) *Job {
	job := E2ETest(k8sVersion)

	job.Name = job.Name + "-issuers-venafi-tpp"
	job.Annotations["description"] = "Runs the E2E tests with 'Venafi TPP' in name"

	job.Labels = make(map[string]string)

	addDefaultE2EVolumeLabels(job)
	addDindLabel(job)
	addMakeVolumesLabel(job)
	addRetryFlakesLabel(job)
	addServiceAccountLabel(job)
	addVenafiTPPLabels(job)

	return job
}

// E2ETestVenafiCloud generates a test which runs end-to-end tests only focusing on Venafi Cloud.
// This runs inside a container and so requires additional permissions.
func E2ETestVenafiCloud(k8sVersion string) *Job {
	job := E2ETest(k8sVersion)

	job.Name = job.Name + "-issuers-venafi-cloud"
	job.Annotations["description"] = "Runs the E2E tests with 'Venafi Cloud' in name"

	job.Labels = make(map[string]string)

	addDefaultE2EVolumeLabels(job)
	addDindLabel(job)
	addMakeVolumesLabel(job)
	addRetryFlakesLabel(job)
	addServiceAccountLabel(job)
	addVenafiCloudLabels(job)

	return job
}

// E2ETestVenafiBoth generates a test which runs end-to-end tests focusing on
// both Venafi TPP and Venafi Cloud.
// This runs inside a container and so requires additional permissions.
func E2ETestVenafiBoth(k8sVersion string) *Job {
	job := E2ETest(k8sVersion)

	job.Name = job.Name + "-issuers-venafi"
	job.Annotations["description"] = "Runs Venafi (VaaS and TPP) e2e tests"

	job.Labels = make(map[string]string)

	addDefaultE2EVolumeLabels(job)
	addDindLabel(job)
	addMakeVolumesLabel(job)
	addRetryFlakesLabel(job)
	addServiceAccountLabel(job)
	addVenafiBothLabels(job)

	return job
}

// E2ETestFeatureGatesDisabled generates a test which runs e2e tests with feature gates disabled
func E2ETestFeatureGatesDisabled(k8sVersion string) *Job {
	job := E2ETest(k8sVersion)

	job.Name = job.Name + "-feature-gates-disabled"
	job.Annotations["description"] = "Runs the E2E tests with all feature gates disabled"

	job.Labels = make(map[string]string)

	addCloudflareCredentialsLabel(job)
	addDefaultE2EVolumeLabels(job)
	addDindLabel(job)
	addDisableFeatureGatesLabel(job)
	addGinkgoSkipDefaultLabel(job)
	addMakeVolumesLabel(job)
	addRetryFlakesLabel(job)
	addServiceAccountLabel(job)

	return job
}

// UpgradeTest generates a test which tests an upgrade from the latest released version
// of cert-manager to the version specified by the test ref / branch. This test runs
// inside a container and so requires additional privileges.
func UpgradeTest(k8sVersion string) *Job {
	nameVersion := strings.ReplaceAll(k8sVersion, ".", "-")

	job := jobTemplate(
		"e2e-v"+nameVersion+"-upgrade",
		"Runs cert-manager upgrade from latest published release",
		addServiceAccountLabel,
		addDefaultE2EVolumeLabels,
		addDindLabel,
		addMakeVolumesLabel,
		addMaxConcurrency(4),
	)

	k8sVersionArg := fmt.Sprintf("K8S_VERSION=%s", k8sVersion)

	job.Spec.Containers = []Container{
		{
			Image: CommonTestImage,
			Args: []string{
				"runner",
				"make",
				k8sVersionArg,
				"vendor-go",
				"test-upgrade",
			},
			Resources: ContainerResources{
				Requests: ContainerResourceRequest{
					CPU:    "3500m",
					Memory: "12Gi",
				},
			},
			SecurityContext: &SecurityContext{
				Privileged: true,
				Capabilities: &SecurityContextCapabilities{
					Add: []string{"SYS_ADMIN"},
				},
			},
		},
	}

	return job
}

// TrivyTest generates a test which runs a Trivy scan of a built container image which matches the given name.
// Note that there's also a "make trivy-scan-all" target, but this will fail as soon as one of its dependencies fails,
// so e.g. if there's a vuln in the "controller" container we might never scan "ctl" container.
// Instead, we generate a test for each container so it's obvious which ones have failures and it's easier to get results
// for each container
func TrivyTest(containerName string) *Job {
	containerName = strings.ToLower(containerName)

	job := jobTemplate(
		fmt.Sprintf("trivy-test-%s", containerName),
		fmt.Sprintf("Runs a Trivy scan against the %s container", containerName),
		addServiceAccountLabel,
		addMakeVolumesLabel,
		addMaxConcurrency(2),
	)

	makeJobs, cpuRequest := calculateMakeConcurrency("1000m")

	job.Spec.Containers = []Container{
		{
			Image: CommonTestImage,
			Args: []string{
				"runner",
				"make",
				makeJobs,
				"vendor-go",
				fmt.Sprintf("trivy-scan-%s", containerName),
			},
			Resources: ContainerResources{
				Requests: ContainerResourceRequest{
					CPU:    cpuRequest,
					Memory: "2Gi",
				},
			},
		},
	}

	return job
}
