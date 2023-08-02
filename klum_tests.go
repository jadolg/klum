package main

import (
	"github.com/jadolg/klum/pkg/controllers/user"
	"github.com/jadolg/klum/pkg/generated/controllers/klum.cattle.io"
	"github.com/jadolg/klum/pkg/metrics"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/rancher/wrangler/pkg/start"
	"k8s.io/client-go/discovery"
	"testing"

	"context"
	"github.com/jadolg/klum/pkg/crd"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/core"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/rbac"
)

func TestMain(m *testing.M) {
	kubeConfig = "~/.kube/config"

	err := main

	if err != nil {
		m.Run()
	}
}

// Test that the <-ctx.Done() function can successfully wait for the context to be done.
func TestWait(t *testing.T) {
	ctx := signals.SetupSignalContext()
	<-ctx.Done()
}

// Test that the crd.Create function can successfully create the CRDs.
func TestCrdCreate(t *testing.T) {
	ctx := signals.SetupSignalContext()
	restConfig, err := kubeconfig.GetNonInteractiveClientConfig(kubeConfig).ClientConfig()
	if err != nil {
		t.Errorf("Error getting config: %v", err)
	}
	if err := crd.Create(ctx, restConfig); err != nil {
		t.Errorf("Error creating CRDs: %v", err)
	}
}

func TestNewFactoryFromConfig(t *testing.T) {
	restConfig, err := kubeconfig.GetNonInteractiveClientConfig(kubeConfig).ClientConfig()
	if err != nil {
		t.Errorf("Error getting client config: %v", err)
	}

	_, err = core.NewFactoryFromConfig(restConfig)
	if err != nil {
		t.Errorf("Error getting core client: %v", err)
	}

	_, err = klum.NewFactoryFromConfigWithNamespace(restConfig, cfg.Namespace)
	if err != nil {
		t.Errorf("Error getting klum client: %v", err)
	}

	if err != nil {
		t.Errorf("Error creating klum factory: %v", err)
	}
}

func TestRegister(t *testing.T) {
	restConfig, err := kubeconfig.GetNonInteractiveClientConfig(kubeConfig).ClientConfig()
	if err != nil {
		t.Errorf("Error getting client config: %v", err)
	}

	ctx := context.Background()
	cfg := user.Config{
		Namespace: "default",
	}
	klum, err := klum.NewFactoryFromConfigWithNamespace(restConfig, cfg.Namespace)
	if err != nil {
		t.Errorf("Error getting klum client: %v", err)
	}

	core, err := core.NewFactoryFromConfig(restConfig)
	if err != nil {
		t.Fatalf("Error getting core client: %v", err)
	}

	rbac, err := rbac.NewFactoryFromConfig(restConfig)
	if err != nil {
		t.Errorf("Error getting rbac client: %v", err)
	}

	apply, err := apply.NewForConfig(restConfig)
	if err != nil {
		t.Errorf("Error creating apply: %v", err)
	}

	// Create a Kubernetes discovery client to get server version.
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		t.Fatalf("Error getting discovery client: %v", err)
	}

	// Get the Kubernetes server version.
	k8sversion, err := discoveryClient.ServerVersion()
	if err != nil {
		t.Fatalf("Error getting server version: %v", err)
	}

	user.Register(ctx,
		cfg,
		apply,
		core.Core().V1().ServiceAccount(),
		rbac.Rbac().V1().ClusterRoleBinding(),
		rbac.Rbac().V1().RoleBinding(),
		core.Core().V1().Secret(),
		klum.Klum().V1alpha1().Kubeconfig(),
		klum.Klum().V1alpha1().User(),
		klum.Klum().V1alpha1().UserSyncGithub(),
		k8sversion,
	)

	if err != nil {
		t.Errorf("Error registering controller: %v", err)
	}
}

// Test that the start.All function can successfully start all of the controllers.
func TestStartAll(t *testing.T) {
	restConfig, err := kubeconfig.GetNonInteractiveClientConfig(kubeConfig).ClientConfig()
	ctx := signals.SetupSignalContext()
	core, err := core.NewFactoryFromConfig(restConfig)
	if err != nil {
		t.Errorf("Error getting core client: %v", err)
	}
	klum, err := klum.NewFactoryFromConfigWithNamespace(restConfig, cfg.Namespace)
	if err != nil {
		t.Errorf("Error getting klum client: %v", err)
	}
	rbac, err := rbac.NewFactoryFromConfig(restConfig)
	if err != nil {
		t.Errorf("Error getting rbac client: %v", err)
	}
	if err := start.All(ctx, 2, klum, core, rbac); err != nil {
		t.Errorf("Error starting controllers: %v", err)
	}
}

func TestUserRegisterUserDoesNotExist(t *testing.T) {
	restConfig, err := kubeconfig.GetNonInteractiveClientConfig(kubeConfig).ClientConfig()
	if err != nil {
		t.Fatalf("Error getting config: %v", err)
	}

	// Create a signal context for handling signals.
	ctx := signals.SetupSignalContext()

	// Create a Kubernetes discovery client to get server version.
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		t.Fatalf("Error getting discovery client: %v", err)
	}

	// Get the Kubernetes server version.
	k8sversion, err := discoveryClient.ServerVersion()
	if err != nil {
		t.Fatalf("Error getting server version: %v", err)
	}

	// Create the user configuration with the desired namespace.
	cfg := user.Config{
		Namespace: "default",
	}

	// Create Kubernetes clients for different resources.
	core, err := core.NewFactoryFromConfig(restConfig)
	if err != nil {
		t.Fatalf("Error getting core client: %v", err)
	}

	applyClient, err := apply.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("Error getting apply client: %v", err)
	}

	rbac, err := rbac.NewFactoryFromConfig(restConfig)
	if err != nil {
		t.Fatalf("Error getting rbac client: %v", err)
	}

	klum, err := klum.NewFactoryFromConfigWithNamespace(restConfig, cfg.Namespace)
	if err != nil {
		t.Fatalf("Error getting klum client: %v", err)
	}

	user.Register(ctx,
		cfg,
		applyClient,
		core.Core().V1().ServiceAccount(),
		rbac.Rbac().V1().ClusterRoleBinding(),
		rbac.Rbac().V1().RoleBinding(),
		core.Core().V1().Secret(),
		klum.Klum().V1alpha1().Kubeconfig(),
		klum.Klum().V1alpha1().User(),
		klum.Klum().V1alpha1().UserSyncGithub(),
		k8sversion,
	)

	// Register the user with the required clients and resources.
	if cfg.MetricsPort != 0 {
		go metrics.StartMetricsServer(cfg.MetricsPort)
	}

	if err := start.All(ctx, 2, klum, core, rbac); err != nil {
		t.Fatalf("Error registering user: %v", err)
	}
}
