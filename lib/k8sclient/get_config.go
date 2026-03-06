package k8sclient

import (
	"flag"
	"fmt"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func GetConfig() (config *rest.Config, err error) {
	var inClusterErr, outOfClusterErr error

	config, inClusterErr = GetConfigInCluster()
	if inClusterErr != nil {
		config, outOfClusterErr = GetConfigOutOfCluster()
		if outOfClusterErr != nil {
			err = fmt.Errorf("GetConfigInCluster() failed: %v; GetConfigOutOfCluster() failed: %v", inClusterErr, outOfClusterErr)
			return nil, err
		}
	}
	return
}

func GetConfigInCluster() (*rest.Config, error) {
	return rest.InClusterConfig()
}

func GetConfigOutOfCluster() (*rest.Config, error) {
	var kubeconfigCliArgument *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfigCliArgument = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfigCliArgument = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	return clientcmd.BuildConfigFromFlags("", *kubeconfigCliArgument)
}