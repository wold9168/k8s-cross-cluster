package main

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func main() {
	// Authentication
	config, err := GetConfig()
	if err != nil {
		klog.Error("Authentication failed due to", err.Error())
		panic(err.Error())
	}
	// 使用上述配置创建一个 Kubernetes 客户端集（clientset），可用于访问所有 Kubernetes API 组
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("Creating clientset failed due to", err.Error())
		panic(err.Error())
	}

	for {
		// 调用 CoreV1 API 的 ConfigMaps 接口，在所有命名空间（传入空字符串 "" 表示全部）中列出 configMaps
		configMaps, err := clientset.CoreV1().ConfigMaps("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			// 若 List 请求失败（如权限不足、API 不可用等），则 panic
			klog.Error("Requesting configMaps failed due to", err.Error())
			panic(err.Error())
			// TODO: 添加事前的鉴权检查，应该直接检查当前鉴权上下文是否支持读写 ConfigMaps
		}
		// 打印当前集群中 configMaps 的总数
		klog.Info("There are %d configMaps in the cluster\n", len(configMaps.Items))

		// 演示如何处理特定 configMap 获取操作中的错误：
		// 尝试从 "default" 命名空间中获取名为 "example-xxxxx" 的 configMap
		var namespace = "default"
		var configMapName = "example"
		_, err = clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
		// 如果错误是“未找到”（HTTP 404），则说明该 configMap 不存在
		if errors.IsNotFound(err) {
			klog.Error("ConfigMap example-xxxxx not found in default namespace\n")
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			// 如果错误是 Kubernetes API 返回的状态错误（如 403、500 等），则提取详细错误信息
			klog.Errorf("Error getting configMap %v\n", statusError.ErrStatus.Message)
		} else if err != nil {
			// 其他非 nil 错误（如网络问题、上下文取消等），直接 panic
			panic(err.Error())
		} else {
			// 如果无错误，说明成功获取到 configMap
			klog.Error("Found example-xxxxx configMap in default namespace\n")
		}

		// 每次循环后暂停 10 秒，避免对 API Server 造成过大压力
		time.Sleep(10 * time.Second)
	}
}
