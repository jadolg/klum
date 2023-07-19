package user

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/jadolg/klum/pkg/github"

	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/version"

	klum "github.com/jadolg/klum/pkg/apis/klum.cattle.io/v1alpha1"
	"github.com/jadolg/klum/pkg/generated/controllers/klum.cattle.io/v1alpha1"
	v1controller "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	rbaccontroller "github.com/rancher/wrangler-api/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/generic"
	name2 "github.com/rancher/wrangler/pkg/name"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type Config struct {
	Namespace          string
	ContextName        string
	Server             string
	CA                 string
	DefaultClusterRole string
	GithubURL          string
	GithubToken        string
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
	userSync v1alpha1.UserSyncController,
	k8sversion *version.Info) {

	h := &handler{
		cfg:             cfg,
		apply:           apply.WithCacheTypes(kconfig),
		serviceAccounts: serviceAccount.Cache(),
		k8sversion:      k8sversion,
		kconfig:         kconfig,
		kuser:           user,
		kuserSync:       userSync,
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

	v1alpha1.RegisterUserSyncGeneratingHandler(
		ctx,
		userSync,
		apply,
		"",
		"klum-usersync",
		h.OnUserSyncChange,
		&generic.GeneratingHandlerOptions{
			AllowClusterScoped: true,
		},
	)

	secrets.OnChange(ctx, "klum-secret", h.OnSecretChange)
	kconfig.OnChange(ctx, "klum-kconfig", h.OnKubeconfigChange)
	userSync.OnRemove(ctx, "klum-usersync", h.OnUserSyncRemove)
}

type handler struct {
	cfg             Config
	apply           apply.Apply
	serviceAccounts v1controller.ServiceAccountCache
	k8sversion      *version.Info
	kuser           v1alpha1.UserController
	kconfig         v1alpha1.KubeconfigController
	kuserSync       v1alpha1.UserSyncController
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
			log.Warning(err)
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

func getUserNameForSecret(secret *v1.Secret, h *handler) (string, *v1.Secret, error, bool) {
	if secret == nil {
		return "", nil, nil, true
	}

	if secret.Type != v1.SecretTypeServiceAccountToken {
		return "", secret, nil, true
	}

	sa, err := h.serviceAccounts.Get(secret.Namespace, secret.Annotations["kubernetes.io/service-account.name"])
	if errors.IsNotFound(err) {
		return "", secret, nil, true
	} else if err != nil {
		return "", secret, nil, true
	}

	if sa.UID != types.UID(secret.Annotations["kubernetes.io/service-account.uid"]) {
		return "", secret, nil, true
	}

	userName := sa.Annotations["klum.cattle.io/user"]
	if userName == "" {
		return "", secret, nil, true
	}

	return userName, nil, nil, false
}

func (h *handler) OnSecretChange(key string, secret *v1.Secret) (*v1.Secret, error) {
	userName, sec, err, done := getUserNameForSecret(secret, h)
	if done {
		return sec, err
	}

	ca := h.cfg.CA
	if ca == "" {
		ca = base64.StdEncoding.EncodeToString(secret.Data["ca.crt"])
	}
	token := string(secret.Data["token"])

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
						Name: h.cfg.ContextName,
						Context: klum.Context{
							Cluster:  h.cfg.ContextName,
							AuthInfo: userName,
						},
					},
				},
				CurrentContext: h.cfg.ContextName,
			},
		})
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
	userSyncs, err := h.kuserSync.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, userSync := range userSyncs.Items {
		if userSync.Spec.User == kubeconfig.Name {
			log.Infof("Synchronizing credentials for %s", userSync.Name)
			h.kuserSync.Enqueue(userSync.Name)
		}
	}
	return kubeconfig, nil
}

func (h *handler) OnUserSyncChange(sync *klum.UserSync, s klum.UserSyncStatus) ([]runtime.Object, klum.UserSyncStatus, error) {
	if sync == nil {
		return nil, setSyncReady(s, false, nil), nil
	}
	if h.cfg.GithubToken != "" {
		kubeconfig, err := h.kconfig.Get(sync.Spec.User, metav1.GetOptions{})
		if err != nil {
			return nil, setSyncReady(s, false, err), err
		}

		if kubeconfig != nil {
			err = github.UploadKubeconfig(sync, kubeconfig, h.cfg.GithubURL, h.cfg.GithubToken)
			if err != nil {
				return nil, setSyncReady(s, false, err), err
			}

			_, err := h.kuserSync.Update(sync)
			if err != nil {
				return nil, setSyncReady(s, false, err), err
			}
		} else {
			return nil, setSyncReady(s, false, err), fmt.Errorf("kubeconfig for user %s is not yet ready", sync.Spec.User)
		}
	} else {
		log.WithFields(log.Fields{
			"usersync": sync.Name,
		}).Warning("Github Synchronization is disabled but UserSync objects are created")
		err := fmt.Errorf("GitHub Synchronization is disabled in klum")
		return nil, setSyncReady(s, false, err), nil
	}

	return []runtime.Object{}, setSyncReady(s, true, nil), nil
}

func (h *handler) OnUserSyncRemove(s string, sync *klum.UserSync) (*klum.UserSync, error) {
	if sync == nil {
		return nil, nil
	}
	if h.cfg.GithubToken != "" {
		err := github.DeleteKubeconfig(sync, h.cfg.GithubURL, h.cfg.GithubToken)
		if err != nil {
			return nil, err
		}
	} else {
		log.Warning("Github Synchronization is disabled but UserSync objects are created")
	}

	return nil, nil
}

func setReady(status klum.UserStatus, ready bool) klum.UserStatus {
	// dumb hack to set condition, should really make this easier
	user := &klum.User{Status: status}
	klum.UserReadyCondition.SetStatusBool(user, ready)
	return user.Status
}

func setSyncReady(status klum.UserSyncStatus, ready bool, err error) klum.UserSyncStatus {
	userSync := &klum.UserSync{Status: status}
	klum.UserSyncReadyCondition.SetStatusBool(userSync, ready)
	if err != nil {
		klum.UserSyncReadyCondition.SetError(userSync, err.Error(), err)
	}
	return userSync.Status
}
