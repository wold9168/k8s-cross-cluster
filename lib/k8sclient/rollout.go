package k8sclient

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// RolloutDeployment triggers a rollout of the specified deployment by adding a timestamp annotation
func RolloutDeployment(clientset kubernetes.Interface, ctx context.Context, namespace, deploymentName string) error {
	klog.Infof("Starting rollout for deployment %s in namespace %s", deploymentName, namespace)

	// Get the deployment
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment %s in namespace %s: %w", deploymentName, namespace, err)
	}

	// Check if pod template annotations exist
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}

	// Add a timestamp annotation to trigger the rollout
	timestamp := time.Now().Format(time.RFC3339)
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = timestamp

	// Update the deployment
	_, err = clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment %s in namespace %s: %w", deploymentName, namespace, err)
	}

	klog.Infof("Successfully triggered rollout for deployment %s in namespace %s with annotation timestamp %s", deploymentName, namespace, timestamp)
	return nil
}
