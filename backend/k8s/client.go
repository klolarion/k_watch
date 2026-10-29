package k8s

import (
	"fmt"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	cfgOnce sync.Once
	cfg     *rest.Config
	cfgErr  error

	csOnce sync.Once
	cs     *kubernetes.Clientset

	metricsOnce sync.Once
	metricsCS   *metricsv1beta1.Clientset
)

func restConfig() (*rest.Config, error) {
	cfgOnce.Do(func() {
		c, err := rest.InClusterConfig()
		if err != nil {
			c, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		}
		if err != nil {
			cfgErr = err
			return
		}
		c.QPS = 50
		c.Burst = 100
		cfg = c
	})
	return cfg, cfgErr
}

func Client() *kubernetes.Clientset {
	csOnce.Do(func() {
		c, err := restConfig()
		if err != nil {
			return
		}
		client, err := kubernetes.NewForConfig(c)
		if err != nil {
			cfgErr = err
			return
		}
		cs = client
	})
	if cs == nil {
		if cfgErr != nil {
			panic(cfgErr)
		}
		panic("kubernetes client is not configured")
	}
	return cs
}

func MetricsClient() *metricsv1beta1.Clientset {
	metricsOnce.Do(func() {
		c, err := restConfig()
		if err != nil {
			fmt.Printf("metrics config error: %v\n", err)
			return
		}
		client, err := metricsv1beta1.NewForConfig(c)
		if err != nil {
			fmt.Printf("metrics client error: %v\n", err)
			return
		}
		metricsCS = client
	})
	return metricsCS
}
