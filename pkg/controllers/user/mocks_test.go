package user

import (
	"context"
	"time"

	klum "github.com/jadolg/klum/pkg/apis/klum.cattle.io/v1alpha1"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/apply/injectors"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/objectset"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	v1controller "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
)

// --- MockUserController ---

type MockUserController struct {
	users       map[string]*klum.User
	EnqueuedIDs []string
}

func NewMockUserController() *MockUserController {
	return &MockUserController{
		users:       make(map[string]*klum.User),
		EnqueuedIDs: []string{},
	}
}

func (m *MockUserController) Get(name string, options metav1.GetOptions) (*klum.User, error) {
	if user, ok := m.users[name]; ok {
		return user.DeepCopy(), nil
	}
	return nil, errors.NewNotFound(schema.GroupResource{Group: "klum.cattle.io", Resource: "users"}, name)
}

func (m *MockUserController) Update(obj *klum.User) (*klum.User, error) {
	m.users[obj.Name] = obj.DeepCopy()
	return obj.DeepCopy(), nil
}

func (m *MockUserController) AddUser(user *klum.User) {
	m.users[user.Name] = user.DeepCopy()
}

func (m *MockUserController) Enqueue(name string) {
	m.EnqueuedIDs = append(m.EnqueuedIDs, name)
}

func (m *MockUserController) EnqueueAfter(name string, duration time.Duration) {}

// Unused interface methods
func (m *MockUserController) Create(obj *klum.User) (*klum.User, error) { panic("not implemented") }
func (m *MockUserController) UpdateStatus(obj *klum.User) (*klum.User, error) {
	panic("not implemented")
}
func (m *MockUserController) Delete(name string, options *metav1.DeleteOptions) error {
	panic("not implemented")
}
func (m *MockUserController) List(opts metav1.ListOptions) (*klum.UserList, error) {
	panic("not implemented")
}
func (m *MockUserController) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("not implemented")
}
func (m *MockUserController) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*klum.User, error) {
	panic("not implemented")
}
func (m *MockUserController) Informer() cache.SharedIndexInformer       { panic("not implemented") }
func (m *MockUserController) GroupVersionKind() schema.GroupVersionKind { panic("not implemented") }
func (m *MockUserController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m *MockUserController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m *MockUserController) Updater() generic.Updater { panic("not implemented") }
func (m *MockUserController) OnChange(ctx context.Context, name string, sync generic.ObjectHandler[*klum.User]) {
}
func (m *MockUserController) OnRemove(ctx context.Context, name string, sync generic.ObjectHandler[*klum.User]) {
}
func (m *MockUserController) Cache() generic.NonNamespacedCacheInterface[*klum.User] {
	panic("not implemented")
}
func (m *MockUserController) WithImpersonation(impersonate rest.ImpersonationConfig) (generic.NonNamespacedClientInterface[*klum.User, *klum.UserList], error) {
	panic("not implemented")
}

// --- MockKubeconfigController ---

type MockKubeconfigController struct {
	kubeconfigs map[string]*klum.Kubeconfig
}

func NewMockKubeconfigController() *MockKubeconfigController {
	return &MockKubeconfigController{
		kubeconfigs: make(map[string]*klum.Kubeconfig),
	}
}

func (m *MockKubeconfigController) Get(name string, options metav1.GetOptions) (*klum.Kubeconfig, error) {
	if kc, ok := m.kubeconfigs[name]; ok {
		return kc.DeepCopy(), nil
	}
	return nil, errors.NewNotFound(schema.GroupResource{Group: "klum.cattle.io", Resource: "kubeconfigs"}, name)
}

func (m *MockKubeconfigController) Delete(name string, options *metav1.DeleteOptions) error {
	if _, ok := m.kubeconfigs[name]; ok {
		delete(m.kubeconfigs, name)
		return nil
	}
	return errors.NewNotFound(schema.GroupResource{Group: "klum.cattle.io", Resource: "kubeconfigs"}, name)
}

func (m *MockKubeconfigController) AddKubeconfig(kc *klum.Kubeconfig) {
	m.kubeconfigs[kc.Name] = kc.DeepCopy()
}

func (m *MockKubeconfigController) EnqueueAfter(name string, duration time.Duration) {}

// Unused interface methods
func (m *MockKubeconfigController) Create(obj *klum.Kubeconfig) (*klum.Kubeconfig, error) {
	panic("not implemented")
}
func (m *MockKubeconfigController) Update(obj *klum.Kubeconfig) (*klum.Kubeconfig, error) {
	panic("not implemented")
}
func (m *MockKubeconfigController) UpdateStatus(obj *klum.Kubeconfig) (*klum.Kubeconfig, error) {
	panic("not implemented")
}
func (m *MockKubeconfigController) List(opts metav1.ListOptions) (*klum.KubeconfigList, error) {
	panic("not implemented")
}
func (m *MockKubeconfigController) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("not implemented")
}
func (m *MockKubeconfigController) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*klum.Kubeconfig, error) {
	panic("not implemented")
}
func (m *MockKubeconfigController) Informer() cache.SharedIndexInformer {
	panic("not implemented")
}
func (m *MockKubeconfigController) GroupVersionKind() schema.GroupVersionKind {
	panic("not implemented")
}
func (m *MockKubeconfigController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m *MockKubeconfigController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m *MockKubeconfigController) Updater() generic.Updater { panic("not implemented") }
func (m *MockKubeconfigController) OnChange(ctx context.Context, name string, sync generic.ObjectHandler[*klum.Kubeconfig]) {
}
func (m *MockKubeconfigController) OnRemove(ctx context.Context, name string, sync generic.ObjectHandler[*klum.Kubeconfig]) {
}
func (m *MockKubeconfigController) Enqueue(name string) {}
func (m *MockKubeconfigController) Cache() generic.NonNamespacedCacheInterface[*klum.Kubeconfig] {
	panic("not implemented")
}
func (m *MockKubeconfigController) WithImpersonation(impersonate rest.ImpersonationConfig) (generic.NonNamespacedClientInterface[*klum.Kubeconfig, *klum.KubeconfigList], error) {
	panic("not implemented")
}

// --- MockUserSyncGithubController ---

type MockUserSyncGithubController struct {
	usersync    map[string]*klum.UserSyncGithub
	EnqueuedIDs []string
}

func NewMockUserSyncGithubController() *MockUserSyncGithubController {
	return &MockUserSyncGithubController{
		usersync:    make(map[string]*klum.UserSyncGithub),
		EnqueuedIDs: []string{},
	}
}

func (m *MockUserSyncGithubController) List(opts metav1.ListOptions) (*klum.UserSyncGithubList, error) {
	list := &klum.UserSyncGithubList{}
	for _, us := range m.usersync {
		list.Items = append(list.Items, *us.DeepCopy())
	}
	return list, nil
}

func (m *MockUserSyncGithubController) Enqueue(name string) {
	m.EnqueuedIDs = append(m.EnqueuedIDs, name)
}

func (m *MockUserSyncGithubController) Update(obj *klum.UserSyncGithub) (*klum.UserSyncGithub, error) {
	m.usersync[obj.Name] = obj.DeepCopy()
	return obj.DeepCopy(), nil
}

func (m *MockUserSyncGithubController) AddUserSyncGithub(us *klum.UserSyncGithub) {
	m.usersync[us.Name] = us.DeepCopy()
}

func (m *MockUserSyncGithubController) EnqueueAfter(name string, duration time.Duration) {}

// Unused interface methods
func (m *MockUserSyncGithubController) Get(name string, options metav1.GetOptions) (*klum.UserSyncGithub, error) {
	panic("not implemented")
}
func (m *MockUserSyncGithubController) Create(obj *klum.UserSyncGithub) (*klum.UserSyncGithub, error) {
	panic("not implemented")
}
func (m *MockUserSyncGithubController) UpdateStatus(obj *klum.UserSyncGithub) (*klum.UserSyncGithub, error) {
	panic("not implemented")
}
func (m *MockUserSyncGithubController) Delete(name string, options *metav1.DeleteOptions) error {
	panic("not implemented")
}
func (m *MockUserSyncGithubController) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("not implemented")
}
func (m *MockUserSyncGithubController) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*klum.UserSyncGithub, error) {
	panic("not implemented")
}
func (m *MockUserSyncGithubController) Informer() cache.SharedIndexInformer {
	panic("not implemented")
}
func (m *MockUserSyncGithubController) GroupVersionKind() schema.GroupVersionKind {
	panic("not implemented")
}
func (m *MockUserSyncGithubController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m *MockUserSyncGithubController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m *MockUserSyncGithubController) Updater() generic.Updater { panic("not implemented") }
func (m *MockUserSyncGithubController) OnChange(ctx context.Context, name string, sync generic.ObjectHandler[*klum.UserSyncGithub]) {
}
func (m *MockUserSyncGithubController) OnRemove(ctx context.Context, name string, sync generic.ObjectHandler[*klum.UserSyncGithub]) {
}
func (m *MockUserSyncGithubController) Cache() generic.NonNamespacedCacheInterface[*klum.UserSyncGithub] {
	panic("not implemented")
}
func (m *MockUserSyncGithubController) WithImpersonation(impersonate rest.ImpersonationConfig) (generic.NonNamespacedClientInterface[*klum.UserSyncGithub, *klum.UserSyncGithubList], error) {
	panic("not implemented")
}

// --- MockServiceAccountCache ---

type MockServiceAccountCache struct {
	serviceAccounts map[string]*v1.ServiceAccount
}

func NewMockServiceAccountCache() *MockServiceAccountCache {
	return &MockServiceAccountCache{
		serviceAccounts: make(map[string]*v1.ServiceAccount),
	}
}

func (m *MockServiceAccountCache) Get(namespace, name string) (*v1.ServiceAccount, error) {
	key := namespace + "/" + name
	if sa, ok := m.serviceAccounts[key]; ok {
		return sa.DeepCopy(), nil
	}
	return nil, errors.NewNotFound(schema.GroupResource{Group: "", Resource: "serviceaccounts"}, name)
}

func (m *MockServiceAccountCache) List(namespace string, selector labels.Selector) ([]*v1.ServiceAccount, error) {
	var result []*v1.ServiceAccount
	for _, sa := range m.serviceAccounts {
		if sa.Namespace == namespace {
			result = append(result, sa.DeepCopy())
		}
	}
	return result, nil
}

func (m *MockServiceAccountCache) AddIndexer(indexName string, indexer v1controller.ServiceAccountIndexer) {
}
func (m *MockServiceAccountCache) GetByIndex(indexName, key string) ([]*v1.ServiceAccount, error) {
	return nil, nil
}

// --- MockApply ---

type MockApply struct {
	AppliedObjects []runtime.Object
	ApplyError     error
}

func NewMockApply() *MockApply {
	return &MockApply{
		AppliedObjects: []runtime.Object{},
	}
}

func (m *MockApply) Apply(set *objectset.ObjectSet) error {
	return m.ApplyError
}

func (m *MockApply) ApplyObjects(objs ...runtime.Object) error {
	m.AppliedObjects = append(m.AppliedObjects, objs...)
	return m.ApplyError
}

func (m *MockApply) WithCacheTypeFactory(factory apply.InformerFactory) apply.Apply { return m }
func (m *MockApply) WithCacheTypes(igs ...apply.InformerGetter) apply.Apply         { return m }
func (m *MockApply) WithSetID(id string) apply.Apply                                { return m }
func (m *MockApply) WithOwner(obj runtime.Object) apply.Apply                       { return m }
func (m *MockApply) WithSetOwnerReference(enabled, block bool) apply.Apply          { return m }
func (m *MockApply) WithOwnerKey(key string, gvk schema.GroupVersionKind) apply.Apply {
	return m
}
func (m *MockApply) WithInjector(injs ...injectors.ConfigInjector) apply.Apply { return m }
func (m *MockApply) WithInjectorName(injs ...string) apply.Apply               { return m }
func (m *MockApply) WithDynamicLookup() apply.Apply                            { return m }

// Unused interface methods
func (m *MockApply) WithIgnorePreviousApplied() apply.Apply { return m }
func (m *MockApply) WithDiffPatch(gvk schema.GroupVersionKind, namespace, name string, patch []byte) apply.Apply {
	return m
}
func (m *MockApply) WithGVK(gvks ...schema.GroupVersionKind) apply.Apply { return m }
func (m *MockApply) WithPatcher(gvk schema.GroupVersionKind, patcher apply.Patcher) apply.Apply {
	return m
}
func (m *MockApply) WithReconciler(gvk schema.GroupVersionKind, reconciler apply.Reconciler) apply.Apply {
	return m
}
func (m *MockApply) WithStrictCaching() apply.Apply                              { return m }
func (m *MockApply) WithDefaultNamespace(ns string) apply.Apply                  { return m }
func (m *MockApply) WithListerNamespace(ns string) apply.Apply                   { return m }
func (m *MockApply) WithRestrictClusterScoped() apply.Apply                      { return m }
func (m *MockApply) WithNoDelete() apply.Apply                                   { return m }
func (m *MockApply) WithNoDeleteGVK(gvks ...schema.GroupVersionKind) apply.Apply { return m }
func (m *MockApply) WithRateLimiting(ratelimitingQps float32) apply.Apply        { return m }
func (m *MockApply) WithContext(ctx context.Context) apply.Apply                 { return m }
func (m *MockApply) DryRun(objs ...runtime.Object) (apply.Plan, error)           { return apply.Plan{}, nil }
func (m *MockApply) FindOwner(obj runtime.Object) (runtime.Object, error)        { return nil, nil }
func (m *MockApply) PurgeOrphan(obj runtime.Object) error                        { return nil }

// --- Test Helper Functions ---

func boolPtr(b bool) *bool {
	return &b
}

func newTestHandler(cfg Config, kuser *MockUserController, kconfig *MockKubeconfigController, kuserSyncGithub *MockUserSyncGithubController, k8sMinor string) *handler {
	return &handler{
		cfg:             cfg,
		kuser:           kuser,
		kconfig:         kconfig,
		kuserSyncGithub: kuserSyncGithub,
		k8sversion:      &version.Info{Minor: k8sMinor},
		serviceAccounts: NewMockServiceAccountCache(),
		apply:           NewMockApply(),
	}
}

func newTestHandlerWithApply(cfg Config, kuser *MockUserController, kconfig *MockKubeconfigController, kuserSyncGithub *MockUserSyncGithubController, mockApply *MockApply, k8sMinor string) *handler {
	return &handler{
		cfg:             cfg,
		kuser:           kuser,
		kconfig:         kconfig,
		kuserSyncGithub: kuserSyncGithub,
		k8sversion:      &version.Info{Minor: k8sMinor},
		serviceAccounts: NewMockServiceAccountCache(),
		apply:           mockApply,
	}
}
