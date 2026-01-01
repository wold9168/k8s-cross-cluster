package main

import (
	"flag"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func GetConfig() (config *rest.Config, err error) {
	config, err = GetConfigInCluster()
	if err != nil {
		config, err = GetConfigOutOfCluster()
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
