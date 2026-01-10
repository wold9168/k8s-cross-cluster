package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

func GetConfig() (config *rest.Config, err error) {
	var inClusterErr, outOfClusterErr error

	config, inClusterErr = GetConfigInCluster()
	if inClusterErr != nil {
		klog.Infof("GetConfigInCluster() failed: %v, try GetConfigOutOfCluster() now.",inClusterErr)
		config, outOfClusterErr = GetConfigOutOfCluster()
		if outOfClusterErr != nil {
			err = fmt.Errorf("GetConfigInCluster() failed: %v; GetConfigOutOfCluster() failed: %v", inClusterErr, outOfClusterErr)
			return nil, err
		}
	}
	klog.Info("GetConfigInCluster() succeeded, pass GetConfigOutOfCluster()")
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
