package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKubernetesProvisionerClonesAndDeletesAllCoreDeploymentsForLayer(t *testing.T) {
	var mu sync.Mutex
	resources := map[string]map[string]interface{}{
		"world-ek":       testBaseDeployment("world-ek", "0"),
		"world-kalimdor": testBaseDeployment("world-kalimdor", "1"),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		mu.Lock()
		defer mu.Unlock()
		name := r.URL.Path[len("/apis/apps/v1/namespaces/test/deployments"):]
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		switch r.Method {
		case http.MethodGet:
			resource, ok := resources[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(resource)
		case http.MethodPost:
			var resource map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&resource))
			metadata := resource["metadata"].(map[string]interface{})
			resources[metadata["name"].(string)] = resource
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			delete(resources, name)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	provisioner, err := NewKubernetesLayerProvisioner(KubernetesLayerProvisionerConfig{
		Namespace: "test", BaseDeployments: []string{"world-ek", "world-kalimdor"}, NamePrefix: "tc9",
		APIURL: server.URL, Token: "test-token", HTTPClient: server.Client(),
	})
	require.NoError(t, err)
	require.NoError(t, provisioner.EnsureLayer(context.Background(), 1, 3))

	mu.Lock()
	for _, name := range []string{"tc9-world-ek-r1-layer-3", "tc9-world-kalimdor-r1-layer-3"} {
		deployment := resources[name]
		require.NotNil(t, deployment)
		spec := deployment["spec"].(map[string]interface{})
		require.Equal(t, name, spec["selector"].(map[string]interface{})["matchLabels"].(map[string]interface{})["app"])
		container := spec["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})
		require.Equal(t, "3", envValue(container, "LAYER_ID"))
	}
	mu.Unlock()

	require.NoError(t, provisioner.DeleteLayer(context.Background(), 1, 3))
	mu.Lock()
	require.NotContains(t, resources, "tc9-world-ek-r1-layer-3")
	require.NotContains(t, resources, "tc9-world-kalimdor-r1-layer-3")
	mu.Unlock()
}

func testBaseDeployment(name, availableMap string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]interface{}{"name": name},
		"spec": map[string]interface{}{
			"replicas": float64(1), "selector": map[string]interface{}{"matchLabels": map[string]interface{}{"app": name}},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{"labels": map[string]interface{}{"app": name}},
				"spec": map[string]interface{}{"containers": []interface{}{map[string]interface{}{
					"name": "worldserver", "image": "world:latest", "env": []interface{}{
						map[string]interface{}{"name": "LAYER_ID", "value": "1"},
						map[string]interface{}{"name": "AVAILABLE_MAPS", "value": availableMap},
					},
				}}},
			},
		},
	}
}

func envValue(container map[string]interface{}, name string) string {
	for _, raw := range container["env"].([]interface{}) {
		entry := raw.(map[string]interface{})
		if entry["name"] == name {
			return entry["value"].(string)
		}
	}
	return ""
}
