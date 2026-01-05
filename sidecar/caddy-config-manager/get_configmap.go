package main

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetConfigMap(clientset *kubernetes.Clientset) (*v1.ConfigMapList, error) {
	return clientset.CoreV1().ConfigMaps("").List(context.TODO(), metav1.ListOptions{})
}
