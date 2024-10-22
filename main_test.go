package main

import (
	"context"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestProcessResources(t *testing.T) {
	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "apps", Version: "v1", Resource: "deployments"}: "deploymentsList",
		},
	)

	// Define test cases
	tests := []struct {
		name                    string
		localBackupDir          string
		kmbMetrics              *kmbmetrics
		privateKey              string
		backupResourcesYamlFile string
		expectedResources       []resourceinfo
	}{
		{
			name:                    "Test with sample resources",
			localBackupDir:          "test-backup-dir",
			kmbMetrics:              nil,
			privateKey:              "",
			backupResourcesYamlFile: "test-resources.yaml",
			expectedResources: []resourceinfo{
				{
					Namespace: getNamespacePointer("default"),
					Group:     "apps",
					Version:   "v1",
					Resource:  "deployments",
					Secret:    false,
				},
			},
		},
	}

	// Create a fake resource to be returned by the dynamic client
	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "deployments",
	})
	resource.SetNamespace("default")
	resource.SetName("test-deployment")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write the test YAML file
			err := os.WriteFile(tt.backupResourcesYamlFile, []byte(
				`resources:
  - namespaces: ["default"]
    group: apps
    version: v1
    resource: deployments
    secret: false
`), 0644)
			if err != nil {
				t.Fatalf("Failed to write test YAML file: %v", err)
			}
			defer os.Remove(tt.backupResourcesYamlFile)

			// Add the fake resource to the dynamic client
			gvr := schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			}
			_, err = dynamicClient.Resource(gvr).Namespace("default").Create(context.TODO(), resource, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create fake resource: %v", err)
			}

			// Call the function to test
			resources := processResources(dynamicClient, tt.localBackupDir, tt.kmbMetrics, tt.privateKey, tt.backupResourcesYamlFile)

			// Check if the returned resources match the expected resources
			if len(resources) != len(tt.expectedResources) {
				t.Errorf("Expected %d resources, got %d", len(tt.expectedResources), len(resources))
			}
			for i, expectedResource := range tt.expectedResources {
				if *resources[i].Namespace != *expectedResource.Namespace ||
					resources[i].Group != expectedResource.Group ||
					resources[i].Version != expectedResource.Version ||
					resources[i].Resource != expectedResource.Resource ||
					resources[i].Secret != expectedResource.Secret {
					t.Errorf("Expected resource %v, got %v", expectedResource, resources[i])
				}
			}
		})
	}
}
