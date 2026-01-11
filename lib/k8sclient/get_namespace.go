package k8sclient

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// GetCurrentNamespace gets the current namespace from multiple sources in order of preference:
// 1. Environment variable POD_NAMESPACE
// 2. Service account file (for in-cluster pods)
// 3. Current context in kubeconfig file (for out-of-cluster scenarios)
func GetCurrentNamespace() (string, error) {
	// First try to get namespace from environment variable
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace != "" {
		return namespace, nil
	}

	// Fallback to reading from the service account directory
	// Working in in-cluster Pods
	namespaceBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err == nil {
		namespace = string(namespaceBytes)
		if len(namespace) > 0 && namespace[len(namespace)-1] == '\n' {
			namespace = namespace[:len(namespace)-1]
		}
		return namespace, nil
	}

	// If not running in cluster, try to get namespace from kubeconfig
	namespace, err = getNamespaceFromKubeconfig()
	if err == nil {
		return namespace, nil
	}

	// If all methods fail, return default namespace
	return "default", err
}

// getNamespaceFromKubeconfig reads the namespace from the current context in kubeconfig
func getNamespaceFromKubeconfig() (string, error) {
	// Use the default kubeconfig path
	kubeconfigFile := filepath.Join(homedir.HomeDir(), ".kube", "config")

	// Check if the kubeconfig file exists
	if _, err := os.Stat(kubeconfigFile); os.IsNotExist(err) {
		return "default", err
	}

	// Load the kubeconfig from the file
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigFile},
		&clientcmd.ConfigOverrides{},
	).RawConfig()
	if err != nil {
		return "default", err
	}

	// Get the current context
	currentContext := config.CurrentContext
	if currentContext == "" {
		return "default", nil
	}

	// Get the namespace from the current context
	context := config.Contexts[currentContext]
	if context == nil {
		return "default", nil
	}

	if context.Namespace == "" {
		return "default", nil
	}

	return context.Namespace, nil
}

// getCurrentNamespaceOrProvided returns the provided namespace if not nil, otherwise returns the current namespace
func getCurrentNamespaceOrProvided(namespace *string) string {
	if namespace != nil {
		return *namespace
	}
	ns, _ := GetCurrentNamespace()
	return ns
}