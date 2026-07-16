package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	serviceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	serviceAccountCAPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

type KubernetesLayerProvisionerConfig struct {
	Namespace       string
	BaseDeployments []string
	NamePrefix      string
	APIURL          string
	Token           string
	HTTPClient      *http.Client
}

type kubernetesLayerProvisioner struct {
	namespace       string
	baseDeployments []string
	namePrefix      string
	apiURL          string
	token           string
	client          *http.Client
}

func NewKubernetesLayerProvisioner(config KubernetesLayerProvisionerConfig) (LayerProvisioner, error) {
	if len(config.BaseDeployments) == 0 {
		return nil, fmt.Errorf("kubernetes layer provisioner requires at least one base deployment")
	}
	if config.Namespace == "" {
		config.Namespace = "default"
	}
	if config.NamePrefix == "" {
		config.NamePrefix = "tc9"
	}
	if config.APIURL == "" {
		host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS")
		if host == "" || port == "" {
			return nil, fmt.Errorf("kubernetes service environment is unavailable")
		}
		config.APIURL = "https://" + host + ":" + port
	}
	if config.Token == "" {
		token, err := os.ReadFile(serviceAccountTokenPath)
		if err != nil {
			return nil, fmt.Errorf("read kubernetes service account token: %w", err)
		}
		config.Token = strings.TrimSpace(string(token))
	}
	if config.HTTPClient == nil {
		ca, err := os.ReadFile(serviceAccountCAPath)
		if err != nil {
			return nil, fmt.Errorf("read kubernetes service account CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("parse kubernetes service account CA")
		}
		config.HTTPClient = &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
	}
	return &kubernetesLayerProvisioner{
		namespace: config.Namespace, baseDeployments: config.BaseDeployments,
		namePrefix: config.NamePrefix, apiURL: strings.TrimRight(config.APIURL, "/"),
		token: config.Token, client: config.HTTPClient,
	}, nil
}

func (k *kubernetesLayerProvisioner) EnsureLayer(ctx context.Context, realmID, layerID uint32) error {
	for _, baseName := range k.baseDeployments {
		targetName := k.deploymentName(baseName, realmID, layerID)
		status, _, err := k.request(ctx, http.MethodGet, k.deploymentPath(targetName), nil)
		if err != nil {
			return err
		}
		if status == http.StatusOK {
			continue
		}
		if status != http.StatusNotFound {
			return fmt.Errorf("check layer deployment %s: kubernetes returned %d", targetName, status)
		}

		status, body, err := k.request(ctx, http.MethodGet, k.deploymentPath(baseName), nil)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("read base deployment %s: kubernetes returned %d: %s", baseName, status, body)
		}
		var base map[string]interface{}
		if err := json.Unmarshal(body, &base); err != nil {
			return fmt.Errorf("decode base deployment %s: %w", baseName, err)
		}
		clone, err := cloneDeploymentForLayer(base, targetName, k.namespace, realmID, layerID)
		if err != nil {
			return fmt.Errorf("clone base deployment %s: %w", baseName, err)
		}
		status, response, err := k.request(ctx, http.MethodPost, k.deploymentsPath(), clone)
		if err != nil {
			return err
		}
		if status != http.StatusCreated && status != http.StatusConflict {
			return fmt.Errorf("create layer deployment %s: kubernetes returned %d: %s", targetName, status, response)
		}
	}
	return nil
}

func (k *kubernetesLayerProvisioner) DeleteLayer(ctx context.Context, realmID, layerID uint32) error {
	for _, baseName := range k.baseDeployments {
		name := k.deploymentName(baseName, realmID, layerID)
		status, body, err := k.request(ctx, http.MethodDelete, k.deploymentPath(name), map[string]interface{}{
			"apiVersion": "v1", "kind": "DeleteOptions", "propagationPolicy": "Foreground",
		})
		if err != nil {
			return err
		}
		if status != http.StatusOK && status != http.StatusAccepted && status != http.StatusNotFound {
			return fmt.Errorf("delete layer deployment %s: kubernetes returned %d: %s", name, status, body)
		}
	}
	return nil
}

func cloneDeploymentForLayer(base map[string]interface{}, name, namespace string, realmID, layerID uint32) (map[string]interface{}, error) {
	spec, ok := base["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("deployment has no spec")
	}
	copyBytes, _ := json.Marshal(spec)
	var clonedSpec map[string]interface{}
	if err := json.Unmarshal(copyBytes, &clonedSpec); err != nil {
		return nil, err
	}
	clonedSpec["replicas"] = float64(1)
	labels := map[string]interface{}{"tc9-layer-id": strconv.FormatUint(uint64(layerID), 10), "tc9-realm-id": strconv.FormatUint(uint64(realmID), 10)}
	selector, ok := clonedSpec["selector"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("deployment has no selector")
	}
	template, ok := clonedSpec["template"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("deployment has no pod template")
	}
	metadata, _ := template["metadata"].(map[string]interface{})
	if metadata == nil {
		metadata = make(map[string]interface{})
		template["metadata"] = metadata
	}
	selectorLabels := stringMap(selector, "matchLabels")
	templateLabels := stringMap(metadata, "labels")
	// Make every inherited selector value unique. Merely adding a layer label
	// to the clone would still let the base Deployment's broader selector match
	// cloned pods, leaving two ReplicaSets competing over the same population.
	for key := range selectorLabels {
		selectorLabels[key] = name
		templateLabels[key] = name
	}
	for key, value := range labels {
		selectorLabels[key] = value
		templateLabels[key] = value
	}
	podSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("deployment pod template has no spec")
	}
	containers, ok := podSpec["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		return nil, fmt.Errorf("deployment has no containers")
	}
	for _, raw := range containers {
		container, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		setContainerEnv(container, "LAYER_ID", strconv.FormatUint(uint64(layerID), 10))
	}
	return map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]interface{}{
			"name": name, "namespace": namespace,
			"labels": map[string]interface{}{"app.kubernetes.io/managed-by": "toc9-layer-controller", "tc9-layer-id": strconv.FormatUint(uint64(layerID), 10)},
		},
		"spec": clonedSpec,
	}, nil
}

func stringMap(parent map[string]interface{}, key string) map[string]interface{} {
	m, _ := parent[key].(map[string]interface{})
	if m == nil {
		m = make(map[string]interface{})
		parent[key] = m
	}
	return m
}

func setContainerEnv(container map[string]interface{}, name, value string) {
	env, _ := container["env"].([]interface{})
	for _, raw := range env {
		entry, ok := raw.(map[string]interface{})
		if ok && entry["name"] == name {
			entry["value"] = value
			delete(entry, "valueFrom")
			container["env"] = env
			return
		}
	}
	container["env"] = append(env, map[string]interface{}{"name": name, "value": value})
}

func (k *kubernetesLayerProvisioner) request(ctx context.Context, method, path string, payload interface{}) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, k.apiURL+path, body)
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Authorization", "Bearer "+k.token)
	request.Header.Set("Content-Type", "application/json")
	response, err := k.client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	return response.StatusCode, data, err
}

func (k *kubernetesLayerProvisioner) deploymentsPath() string {
	return "/apis/apps/v1/namespaces/" + k.namespace + "/deployments"
}
func (k *kubernetesLayerProvisioner) deploymentPath(name string) string {
	return k.deploymentsPath() + "/" + name
}
func (k *kubernetesLayerProvisioner) deploymentName(base string, realmID, layerID uint32) string {
	suffix := fmt.Sprintf("-r%d-layer-%d", realmID, layerID)
	prefix := strings.ToLower(strings.Trim(k.namePrefix+"-"+base, "-"))
	if max := 63 - len(suffix); len(prefix) > max {
		prefix = strings.TrimRight(prefix[:max], "-")
	}
	return prefix + suffix
}
