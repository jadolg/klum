package user

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/jadolg/klum/pkg/metrics"

	"github.com/jadolg/klum/pkg/github"

	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/version"

	klum "github.com/jadolg/klum/pkg/apis/klum.cattle.io/v1alpha1"
	"github.com/jadolg/klum/pkg/generated/controllers/klum.cattle.io/v1alpha1"
	v1controller "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	rbaccontroller "github.com/rancher/wrangler-api/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/generic"
	name2 "github.com/rancher/wrangler/v3/pkg/name"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Config struct {
	Namespace          string
	ContextName        string
	Server             string
	CA                 string
	DefaultClusterRole string
	GithubConfig       github.Config
	MetricsPort        int
}

func Register(ctx context.Context,
	cfg Config,
	apply apply.Apply,
	serviceAccount v1controller.ServiceAccountController,
	crb rbaccontroller.ClusterRoleBindingController,
	rb rbaccontroller.RoleBindingController,
	secrets v1controller.SecretController,
	kconfig v1alpha1.KubeconfigController,
	user v1alpha1.UserController,
	userSyncGithub v1alpha1.UserSyncGithubController,
	k8sversion *version.Info) {

	h := &handler{
		cfg:             cfg,
		apply:           apply.WithCacheTypes(kconfig),
		serviceAccounts: serviceAccount.Cache(),
		k8sversion:      k8sversion,
		kconfig:         kconfig,
		kuser:           user,
		kuserSyncGithub: userSyncGithub,
	}

	v1alpha1.RegisterUserGeneratingHandler(ctx,
		user,
		apply.WithCacheTypes(serviceAccount, crb, rb, secrets),
		"",
		"klum-user",
		h.OnUserChange,
		&generic.GeneratingHandlerOptions{
			AllowClusterScoped: true,
		})

	v1alpha1.RegisterUserSyncGithubGeneratingHandler(
		ctx,
		userSyncGithub,
		apply,
		"",
		"klum-usersync",
		h.OnUserSyncGithubChange,
		&generic.GeneratingHandlerOptions{
			AllowClusterScoped: true,
		},
	)

	secrets.OnChange(ctx, "klum-secret", h.OnSecretChange)
	kconfig.OnChange(ctx, "klum-kconfig", h.OnKubeconfigChange)
	userSyncGithub.OnRemove(ctx, "klum-usersync", h.OnUserSyncGithubRemove)
	user.OnRemove(ctx, "klum-user", h.OnUserRemoved)
}

type handler struct {
	cfg             Config
	apply           apply.Apply
	serviceAccounts v1controller.ServiceAccountCache
	k8sversion      *version.Info
	kuser           v1alpha1.UserController
	kconfig         v1alpha1.KubeconfigController
	kuserSyncGithub v1alpha1.UserSyncGithubController
}

func sanitizedVersion(v string) int {
	v = strings.Trim(v, "+")
	intVersion, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("invalid kubernetes minor version: %v", err)
	}
	return intVersion
}

func (h *handler) OnUserChange(user *klum.User, status klum.UserStatus) ([]runtime.Object, klum.UserStatus, error) {
	if user.Spec.Enabled != nil && !*user.Spec.Enabled {
		status = setReady(status, false)
		err := h.removeKubeconfig(user)
		if err != nil {
			log.Error(err)
			metrics.ErrorsTotal.Inc()
		}
		return nil, status, nil
	}

	objs := []runtime.Object{
		&v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      user.Name,
				Namespace: h.cfg.Namespace,
				Annotations: map[string]string{
					"klum.cattle.io/user": user.Name,
				},
			},
		},
	}

	if sanitizedVersion(h.k8sversion.Minor) >= 24 {
		objs = append(objs,
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      user.Name,
					Namespace: h.cfg.Namespace,
					Annotations: map[string]string{
						"kubernetes.io/service-account.name": user.Name,
					},
				},
				Type: v1.SecretTypeServiceAccountToken,
			},
		)
	}

	objs = append(objs, h.getRoles(user)...)

	return objs, setReady(status, true), nil
}

func (h *handler) getRoles(user *klum.User) []runtime.Object {
	subjects := []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      user.Name,
			Namespace: h.cfg.Namespace,
		},
	}

	if len(user.Spec.ClusterRoles) == 0 && len(user.Spec.Roles) == 0 {
		if h.cfg.DefaultClusterRole == "" {
			return nil
		}
		return []runtime.Object{
			&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: name(user.Name, "", h.cfg.DefaultClusterRole, ""),
				},
				Subjects: subjects,
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     h.cfg.DefaultClusterRole,
				},
			},
		}
	}

	var objs []runtime.Object

	for _, clusterRole := range user.Spec.ClusterRoles {
		objs = append(objs, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name(user.Name, "", clusterRole, ""),
			},
			Subjects: subjects,
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     clusterRole,
			},
		})
	}

	for _, role := range user.Spec.Roles {
		if role.Namespace == "" ||
			role.Role == "" && role.ClusterRole == "" {
			continue
		}

		if role.Role != "" {
			rb := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name(user.Name, role.Namespace, "", role.Role),
					Namespace: role.Namespace,
				},
				Subjects: subjects,
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Role",
					Name:     role.Role,
				},
			}
			objs = append(objs, rb)
		}

		if role.ClusterRole != "" {
			rb := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name(user.Name, role.Namespace, role.ClusterRole, ""),
					Namespace: role.Namespace,
				},
				Subjects: subjects,
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     role.ClusterRole,
				},
			}
			objs = append(objs, rb)
		}
	}

	return objs
}

func name(user, namespace, clusterRole, role string) string {
	// this is so we don't get conflicts
	suffix := md5.Sum([]byte(fmt.Sprintf("%s/%s/%s/%s", user, namespace, clusterRole, role)))
	if role == "" {
		role = clusterRole
	}
	return name2.SafeConcatName("klum", user, role, hex.EncodeToString(suffix[:])[:8])
}

func getUserNameForSecret(secret *v1.Secret) string {
	if secret == nil {
		return ""
	}

	if secret.Type != v1.SecretTypeServiceAccountToken {
		return ""
	}

	if klumUserAnnotation, present := secret.Annotations["objectset.rio.cattle.io/id"]; present && klumUserAnnotation == "klum-user" {
		if username, present := secret.Annotations["objectset.rio.cattle.io/owner-name"]; present {
			return username
		}
		return ""
	}
	return ""
}

func getUserByName(name string, h *handler) (*klum.User, error) {
	user, err := h.kuser.Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (h *handler) OnSecretChange(key string, secret *v1.Secret) (*v1.Secret, error) {
	userName := getUserNameForSecret(secret)
	if userName == "" {
		return secret, nil
	}

	ca := h.cfg.CA
	if ca == "" {
		ca = base64.StdEncoding.EncodeToString(secret.Data["ca.crt"])
	}
	token := string(secret.Data["token"])

	contextName := h.cfg.ContextName
	contextNamespace := "default"
	user, err := getUserByName(userName, h)
	if err == nil {
		if err := h.updateUserDefaults(user); err != nil {
			return nil, err
		}
		contextName = user.Spec.Context
		contextNamespace = user.Spec.ContextNamespace
	}

	return secret, h.apply.
		WithOwner(secret).
		WithSetOwnerReference(true, false).
		ApplyObjects(&klum.Kubeconfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: userName,
			},
			Spec: klum.KubeconfigSpec{
				Clusters: []klum.NamedCluster{
					{
						Name: h.cfg.ContextName,
						Cluster: klum.Cluster{
							Server:                   h.cfg.Server,
							CertificateAuthorityData: ca,
						},
					},
				},
				AuthInfos: []klum.NamedAuthInfo{
					{
						Name: userName,
						AuthInfo: klum.AuthInfo{
							Token: token,
						},
					},
				},
				Contexts: []klum.NamedContext{
					{
						Name: contextName,
						Context: klum.Context{
							Cluster:   h.cfg.ContextName,
							AuthInfo:  userName,
							Namespace: contextNamespace,
						},
					},
				},
				CurrentContext: contextName,
			},
		})
}

func (h *handler) updateUserDefaults(user *klum.User) error {
	anyChanges := false

	if user.Spec.Context == "" {
		user.Spec.Context = h.cfg.ContextName
		anyChanges = true
	}

	if user.Spec.ContextNamespace == "" {
		user.Spec.ContextNamespace = getUserDefaultNamespace(user)
		anyChanges = true
	}

	if !anyChanges {
		return nil
	}

	_, err := h.kuser.Update(user)
	return err
}

func getUserDefaultNamespace(user *klum.User) string {
	if user.Spec.ContextNamespace != "" {
		return user.Spec.ContextNamespace
	}
	for _, role := range user.Spec.Roles {
		if role.Namespace != "" {
			return role.Namespace
		}
	}
	return "default"
}

func (h *handler) removeKubeconfig(user *klum.User) error {
	_, err := h.kconfig.Get(user.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = h.kconfig.Delete(user.Name, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (h *handler) OnKubeconfigChange(s string, kubeconfig *klum.Kubeconfig) (*klum.Kubeconfig, error) {
	if kubeconfig == nil {
		return nil, nil
	}
	// ToDo: Check how we can make `spec.user` usable as a field selector
	userSyncsGithub, err := h.kuserSyncGithub.List(metav1.ListOptions{})
	if err != nil {
		metrics.ErrorsTotal.Inc()
		return nil, err
	}
	for _, userSync := range userSyncsGithub.Items {
		if userSync.Spec.User == kubeconfig.Name {
			log.Infof("Synchronizing credentials for %s", userSync.Name)
			h.kuserSyncGithub.Enqueue(userSync.Name)
		}
	}
	return kubeconfig, nil
}

func (h *handler) OnUserSyncGithubChange(syncGithub *klum.UserSyncGithub, s klum.UserSyncStatus) ([]runtime.Object, klum.UserSyncStatus, error) {
	if syncGithub == nil {
		return nil, setSyncGithubReady(s, false, nil), nil
	}
	if h.cfg.GithubConfig.Enabled() {
		kubeconfig, err := h.kconfig.Get(syncGithub.Spec.User, metav1.GetOptions{})
		if err != nil {
			return nil, setSyncGithubReady(s, false, err), err
		}

		if kubeconfig != nil {
			err = github.UploadKubeconfig(syncGithub, kubeconfig, h.cfg.GithubConfig)
			if err != nil {
				metrics.ErrorsTotal.Inc()
				return nil, setSyncGithubReady(s, false, err), err
			}

			_, err := h.kuserSyncGithub.Update(syncGithub)
			if err != nil {
				metrics.ErrorsTotal.Inc()
				return nil, setSyncGithubReady(s, false, err), err
			}
		} else {
			return nil, setSyncGithubReady(s, false, err), fmt.Errorf("kubeconfig for user %s is not yet ready", syncGithub.Spec.User)
		}
	} else {
		log.WithFields(log.Fields{
			"usersync": syncGithub.Name,
		}).Warning("Github Synchronization is disabled but UserSyncGithub objects are created")
		err := fmt.Errorf("GitHub Synchronization is disabled in klum")
		metrics.ErrorsTotal.Inc()
		return nil, setSyncGithubReady(s, false, err), nil
	}

	return []runtime.Object{}, setSyncGithubReady(s, true, nil), nil
}

func (h *handler) OnUserSyncGithubRemove(s string, sync *klum.UserSyncGithub) (*klum.UserSyncGithub, error) {
	if sync == nil {
		return nil, nil
	}
	if h.cfg.GithubConfig.Enabled() {
		err := github.DeleteKubeconfig(sync, h.cfg.GithubConfig)
		if err != nil {
			metrics.ErrorsTotal.Inc()
			return nil, err
		}
	} else {
		log.Warning("Github Synchronization is disabled but UserSyncGithub objects are created")
	}

	return nil, nil
}

func setReady(status klum.UserStatus, ready bool) klum.UserStatus {
	// dumb hack to set condition, should really make this easier
	user := &klum.User{Status: status}
	klum.UserReadyCondition.SetStatusBool(user, ready)
	return user.Status
}

func setSyncGithubReady(status klum.UserSyncStatus, ready bool, err error) klum.UserSyncStatus {
	userSync := &klum.UserSyncGithub{Status: status}
	klum.UserSyncReadyCondition.SetStatusBool(userSync, ready)
	if err != nil {
		metrics.ErrorsTotal.Inc()
		klum.UserSyncReadyCondition.SetError(userSync, err.Error(), err)
	}
	return userSync.Status
}

func (h *handler) OnUserRemoved(key string, user *klum.User) (*klum.User, error) {
	_, err := h.kconfig.Get(user.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return user, nil
		}
		return user, err
	}

	err = h.kconfig.Delete(user.Name, &metav1.DeleteOptions{})
	if err != nil {
		return user, err
	}

	return user, nil
}
