package kube

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/testhelpers"

	"github.com/stretchr/testify/assert"
)

func jobTestLoadRole(assert *assert.Assertions, roleName, manifestName string) *model.InstanceGroup {
	workDir, err := os.Getwd()
	assert.NoError(err)

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests/kube", manifestName)
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	manifest, err := model.LoadRoleManifest(
		manifestPath,
		[]string{releasePath},
		[]string{},
		[]string{},
		filepath.Join(workDir, "../test-assets/bosh-cache"),
		nil)
	if !assert.NoError(err) {
		return nil
	}

	instanceGroup := manifest.LookupInstanceGroup(roleName)
	if !assert.NotNil(instanceGroup, "Failed to find instance group %s", roleName) {
		return nil
	}
	return instanceGroup
}

func TestJobPreFlight(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	instanceGroup := jobTestLoadRole(assert, "pre-role", "jobs.yml")
	if instanceGroup == nil {
		return
	}

	job, err := NewJob(instanceGroup, ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err, "Failed to create job from instance group pre-role") {
		return
	}
	assert.NotNil(job)

	actual, err := RoundtripKube(job)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLSubsetString(assert, `---
		apiVersion: batch/v1
		kind: Job
		metadata:
			name: pre-role
		spec:
			template:
				metadata:
					name: pre-role
				spec:
					containers:
					-
						name: pre-role
					restartPolicy: OnFailure
	`, actual)
}

func TestJobPostFlight(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	instanceGroup := jobTestLoadRole(assert, "post-role", "jobs.yml")
	if instanceGroup == nil {
		return
	}

	job, err := NewJob(instanceGroup, ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err, "Failed to create job from role post-role") {
		return
	}
	assert.NotNil(job)

	actual, err := RoundtripKube(job)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLSubsetString(assert, `---
		apiVersion: batch/v1
		kind: Job
		metadata:
			name: post-role
		spec:
			template:
				metadata:
					name: post-role
				spec:
					containers:
					-
						name: post-role
					restartPolicy: OnFailure
	`, actual)
}

func TestJobWithAnnotations(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	instanceGroup := jobTestLoadRole(assert, "some-group", "job-with-annotation.yml")
	if instanceGroup == nil {
		return
	}

	job, err := NewJob(instanceGroup, ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err, "Failed to create job from instance group pre-role") {
		return
	}
	assert.NotNil(job)

	actual, err := RoundtripKube(job)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLSubsetString(assert, `---
		apiVersion: batch/v1
		kind: Job
		metadata:
			name: some-group
			annotations:
				helm.sh/hook: post-install
	`, actual)
}

func TestJobHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := jobTestLoadRole(assert, "pre-role", "jobs.yml")
	if role == nil {
		return
	}

	job, err := NewJob(role, ExportSettings{
		Opinions:        model.NewEmptyOpinions(),
		CreateHelmChart: true,
		Repository:      "the_repos",
	}, nil)
	if !assert.NoError(err, "Failed to create job from role pre-role") {
		return
	}
	assert.NotNil(job)

	workDir, err := os.Getwd()
	assert.NoError(err)
	fakeTemplateDir := filepath.Join(workDir, "../test-assets/fake-templates")

	// Notes. The variables
	// - Release.Revision
	// - Values.kube.registry.hostname
	// - Values.kube.organization
	// - Values.env.KUBERNETES_CLUSTER_DOMAIN
	// can all be removed without causing an error during render.
	// The output simply gains <no value>, and empty string.
	//
	// TODO: Rework NewJob to make these `required` in the template.
	//       (and add tests demonstrating that)

	config := map[string]interface{}{
		"Capabilities.KubeVersion.Major": "1",
		"Capabilities.KubeVersion.Minor": "6",
		// Fake location for a fake `secrets.yaml`.
		"Template.BasePath":                    fakeTemplateDir,
		"Release.Revision":                     "42",
		"Values.kube.registry.hostname":        "docker.suse.fake",
		"Values.kube.organization":             "splat",
		"Values.env.KUBERNETES_CLUSTER_DOMAIN": "cluster.local",
		"Values.sizing.pre_role.capabilities":  []interface{}{},
	}

	actual, err := RoundtripNode(job, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: batch/v1
		kind: "Job"
		metadata:
			name: "pre-role-42"
		spec:
			template:
				metadata:
					name: "pre-role"
					labels:
						skiff-role-name: "pre-role"
					annotations:
						checksum/config: 08c80ed11902eefef09739d41c91408238bb8b5e7be7cc1e5db933b7c8de65c3
				spec:
					containers:
					-	env:
						-	name: "KUBERNETES_CLUSTER_DOMAIN"
							value: "cluster.local"
						-	name: "KUBERNETES_NAMESPACE"
							valueFrom:
								fieldRef:
									fieldPath: "metadata.namespace"
						image: "docker.suse.fake/splat/the_repos-pre-role:b0668a0daba46290566d99ee97d7b45911a53293"
						lifecycle:
							preStop:
								exec:
									command:
									-	"/opt/fissile/pre-stop.sh"
						livenessProbe: ~
						name: "pre-role"
						ports: ~
						readinessProbe: ~
						resources: ~
						securityContext:
							capabilities:
								add:	~
						volumeMounts: ~
					dnsPolicy: "ClusterFirst"
					imagePullSecrets:
					-	name: "registry-credentials"
					restartPolicy: "OnFailure"
					terminationGracePeriodSeconds: 600
					volumes: ~
	`, actual)
}
