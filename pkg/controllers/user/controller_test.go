package user

import (
	"testing"

	klum "github.com/jadolg/klum/pkg/apis/klum.cattle.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVersion(t *testing.T) {
	assert.Equal(t, 25, sanitizedVersion("25"))
	assert.Equal(t, 25, sanitizedVersion("25+"))
	assert.Equal(t, 25, sanitizedVersion("+25"))
}

func TestName(t *testing.T) {
	// Test that name() produces deterministic output
	n1 := name("testuser", "", "admin", "")
	n2 := name("testuser", "", "admin", "")
	assert.Equal(t, n1, n2, "name() should produce deterministic output")

	// Different inputs should produce different names
	n3 := name("testuser", "namespace", "admin", "")
	assert.NotEqual(t, n1, n3, "different inputs should produce different names")

	// With role vs cluster role
	n4 := name("testuser", "namespace", "", "editor")
	assert.NotEqual(t, n3, n4)

	// Name should contain the user
	assert.Contains(t, n1, "testuser")
}

func TestGetUserNameForSecret(t *testing.T) {
	tests := []struct {
		name     string
		secret   *v1.Secret
		expected string
	}{
		{
			name:     "nil secret returns empty",
			secret:   nil,
			expected: "",
		},
		{
			name: "wrong secret type returns empty",
			secret: &v1.Secret{
				Type: v1.SecretTypeOpaque,
			},
			expected: "",
		},
		{
			name: "missing klum annotation returns empty",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Type: v1.SecretTypeServiceAccountToken,
			},
			expected: "",
		},
		{
			name: "wrong klum annotation value returns empty",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"objectset.rio.cattle.io/id": "other-app",
					},
				},
				Type: v1.SecretTypeServiceAccountToken,
			},
			expected: "",
		},
		{
			name: "valid annotations returns username",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"objectset.rio.cattle.io/id":         "klum-user",
						"objectset.rio.cattle.io/owner-name": "testuser",
					},
				},
				Type: v1.SecretTypeServiceAccountToken,
			},
			expected: "testuser",
		},
		{
			name: "klum annotation present but no owner name returns empty",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"objectset.rio.cattle.io/id": "klum-user",
					},
				},
				Type: v1.SecretTypeServiceAccountToken,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getUserNameForSecret(tt.secret)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetUserDefaultNamespace(t *testing.T) {
	tests := []struct {
		name     string
		user     *klum.User
		expected string
	}{
		{
			name: "returns context namespace when set",
			user: &klum.User{
				Spec: klum.UserSpec{
					ContextNamespace: "custom-ns",
				},
			},
			expected: "custom-ns",
		},
		{
			name: "returns first role namespace when context namespace empty",
			user: &klum.User{
				Spec: klum.UserSpec{
					Roles: []klum.NamespaceRole{
						{Namespace: "role-ns-1"},
						{Namespace: "role-ns-2"},
					},
				},
			},
			expected: "role-ns-1",
		},
		{
			name: "returns default when no namespace configured",
			user: &klum.User{
				Spec: klum.UserSpec{},
			},
			expected: "default",
		},
		{
			name: "skips empty namespace in roles",
			user: &klum.User{
				Spec: klum.UserSpec{
					Roles: []klum.NamespaceRole{
						{Namespace: ""},
						{Namespace: "second-ns"},
					},
				},
			},
			expected: "second-ns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getUserDefaultNamespace(tt.user)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetReady(t *testing.T) {
	status := klum.UserStatus{}

	// Set ready to true
	newStatus := setReady(status, true)
	assert.NotEmpty(t, newStatus.Conditions)

	// Set ready to false
	newStatus = setReady(status, false)
	assert.NotEmpty(t, newStatus.Conditions)
}

func TestOnUserChange_EnabledUser_K8s24Plus(t *testing.T) {
	cfg := Config{
		Namespace:          "klum-system",
		DefaultClusterRole: "cluster-admin",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testuser",
		},
		Spec: klum.UserSpec{},
	}

	objs, status, err := h.OnUserChange(user, klum.UserStatus{})

	require.NoError(t, err)
	assert.NotEmpty(t, status.Conditions)

	// Should create ServiceAccount, Secret (k8s >= 24), and ClusterRoleBinding with default role
	require.Len(t, objs, 3, "expected ServiceAccount, Secret, and ClusterRoleBinding")

	// First object should be ServiceAccount
	sa, ok := objs[0].(*v1.ServiceAccount)
	require.True(t, ok, "first object should be ServiceAccount")
	assert.Equal(t, "testuser", sa.Name)
	assert.Equal(t, "klum-system", sa.Namespace)
	assert.Equal(t, "testuser", sa.Annotations["klum.cattle.io/user"])

	// Second object should be Secret
	secret, ok := objs[1].(*v1.Secret)
	require.True(t, ok, "second object should be Secret")
	assert.Equal(t, "testuser", secret.Name)
	assert.Equal(t, v1.SecretTypeServiceAccountToken, secret.Type)
	assert.Equal(t, "testuser", secret.Annotations["kubernetes.io/service-account.name"])

	// Third object should be ClusterRoleBinding
	crb, ok := objs[2].(*rbacv1.ClusterRoleBinding)
	require.True(t, ok, "third object should be ClusterRoleBinding")
	assert.Equal(t, "cluster-admin", crb.RoleRef.Name)
	assert.Equal(t, "ClusterRole", crb.RoleRef.Kind)
}

func TestOnUserChange_EnabledUser_K8sBelow24(t *testing.T) {
	cfg := Config{
		Namespace:          "klum-system",
		DefaultClusterRole: "cluster-admin",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "23")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testuser",
		},
		Spec: klum.UserSpec{},
	}

	objs, status, err := h.OnUserChange(user, klum.UserStatus{})

	require.NoError(t, err)
	assert.NotEmpty(t, status.Conditions)

	// Should create only ServiceAccount and ClusterRoleBinding (no Secret for k8s < 24)
	require.Len(t, objs, 2, "expected ServiceAccount and ClusterRoleBinding for k8s < 24")

	_, ok := objs[0].(*v1.ServiceAccount)
	require.True(t, ok, "first object should be ServiceAccount")

	_, ok = objs[1].(*rbacv1.ClusterRoleBinding)
	require.True(t, ok, "second object should be ClusterRoleBinding")
}

func TestOnUserChange_DisabledUser(t *testing.T) {
	cfg := Config{
		Namespace: "klum-system",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	// Add kubeconfig that should be deleted
	kconfig.AddKubeconfig(&klum.Kubeconfig{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
	})

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	enabled := false
	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testuser",
		},
		Spec: klum.UserSpec{
			Enabled: &enabled,
		},
	}

	objs, status, err := h.OnUserChange(user, klum.UserStatus{})

	require.NoError(t, err)
	assert.Nil(t, objs, "disabled user should return nil objects")
	assert.NotEmpty(t, status.Conditions)

	// Kubeconfig should have been deleted
	_, err = kconfig.Get("testuser", metav1.GetOptions{})
	assert.Error(t, err, "kubeconfig should have been deleted")
}

func TestOnUserChange_WithClusterRoles(t *testing.T) {
	cfg := Config{
		Namespace: "klum-system",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testuser",
		},
		Spec: klum.UserSpec{
			ClusterRoles: []string{"admin", "view"},
		},
	}

	objs, _, err := h.OnUserChange(user, klum.UserStatus{})

	require.NoError(t, err)
	// ServiceAccount + Secret + 2 ClusterRoleBindings
	require.Len(t, objs, 4)

	// Check ClusterRoleBindings
	adminCRB, ok := objs[2].(*rbacv1.ClusterRoleBinding)
	require.True(t, ok)
	assert.Equal(t, "admin", adminCRB.RoleRef.Name)

	viewCRB, ok := objs[3].(*rbacv1.ClusterRoleBinding)
	require.True(t, ok)
	assert.Equal(t, "view", viewCRB.RoleRef.Name)
}

func TestOnUserChange_WithNamespaceRoles(t *testing.T) {
	cfg := Config{
		Namespace: "klum-system",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testuser",
		},
		Spec: klum.UserSpec{
			Roles: []klum.NamespaceRole{
				{Namespace: "dev", Role: "editor"},
				{Namespace: "prod", ClusterRole: "view"},
			},
		},
	}

	objs, _, err := h.OnUserChange(user, klum.UserStatus{})

	require.NoError(t, err)
	// ServiceAccount + Secret + 2 RoleBindings
	require.Len(t, objs, 4)

	// Check RoleBindings
	rb1, ok := objs[2].(*rbacv1.RoleBinding)
	require.True(t, ok)
	assert.Equal(t, "dev", rb1.Namespace)
	assert.Equal(t, "editor", rb1.RoleRef.Name)
	assert.Equal(t, "Role", rb1.RoleRef.Kind)

	rb2, ok := objs[3].(*rbacv1.RoleBinding)
	require.True(t, ok)
	assert.Equal(t, "prod", rb2.Namespace)
	assert.Equal(t, "view", rb2.RoleRef.Name)
	assert.Equal(t, "ClusterRole", rb2.RoleRef.Kind)
}

func TestOnUserChange_NoRolesNoDefault(t *testing.T) {
	cfg := Config{
		Namespace:          "klum-system",
		DefaultClusterRole: "", // No default
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testuser",
		},
		Spec: klum.UserSpec{},
	}

	objs, _, err := h.OnUserChange(user, klum.UserStatus{})

	require.NoError(t, err)
	// Only ServiceAccount + Secret, no role bindings
	require.Len(t, objs, 2)
}

func TestOnUserRemoved(t *testing.T) {
	cfg := Config{}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	// Add kubeconfig that should be deleted
	kconfig.AddKubeconfig(&klum.Kubeconfig{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
	})

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testuser",
		},
	}

	resultUser, err := h.OnUserRemoved("testuser", user)

	require.NoError(t, err)
	assert.Equal(t, user, resultUser)

	// Kubeconfig should have been deleted
	_, err = kconfig.Get("testuser", metav1.GetOptions{})
	assert.Error(t, err, "kubeconfig should have been deleted")
}

func TestOnUserRemoved_KubeconfigNotFound(t *testing.T) {
	cfg := Config{}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testuser",
		},
	}

	// Should not fail when kubeconfig doesn't exist
	resultUser, err := h.OnUserRemoved("testuser", user)

	require.NoError(t, err)
	assert.Equal(t, user, resultUser)
}

func TestRemoveKubeconfig(t *testing.T) {
	cfg := Config{}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	kconfig.AddKubeconfig(&klum.Kubeconfig{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
	})

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testuser",
		},
	}

	err := h.removeKubeconfig(user)
	require.NoError(t, err)

	_, err = kconfig.Get("testuser", metav1.GetOptions{})
	assert.Error(t, err, "kubeconfig should have been deleted")
}

func TestRemoveKubeconfig_NotFound(t *testing.T) {
	cfg := Config{}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nonexistent",
		},
	}

	err := h.removeKubeconfig(user)
	assert.Error(t, err, "should return error when kubeconfig not found")
}

func TestGetRoles_NoRolesNoDefault(t *testing.T) {
	cfg := Config{
		Namespace:          "klum-system",
		DefaultClusterRole: "",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
		Spec:       klum.UserSpec{},
	}

	objs := h.getRoles(user)
	assert.Nil(t, objs)
}

func TestGetRoles_InvalidRolesSkipped(t *testing.T) {
	cfg := Config{
		Namespace: "klum-system",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
		Spec: klum.UserSpec{
			Roles: []klum.NamespaceRole{
				// Invalid: empty namespace
				{Namespace: "", Role: "admin"},
				// Invalid: empty role and clusterRole
				{Namespace: "dev"},
				// Valid
				{Namespace: "prod", Role: "editor"},
			},
		},
	}

	objs := h.getRoles(user)
	require.Len(t, objs, 1)

	rb, ok := objs[0].(*rbacv1.RoleBinding)
	require.True(t, ok)
	assert.Equal(t, "prod", rb.Namespace)
	assert.Equal(t, "editor", rb.RoleRef.Name)
}

func TestGetRoles_BothRoleAndClusterRole(t *testing.T) {
	cfg := Config{
		Namespace: "klum-system",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
		Spec: klum.UserSpec{
			Roles: []klum.NamespaceRole{
				// Both Role and ClusterRole specified - should create 2 RoleBindings
				{Namespace: "dev", Role: "editor", ClusterRole: "view"},
			},
		},
	}

	objs := h.getRoles(user)
	require.Len(t, objs, 2)

	// First should be for Role
	rb1, ok := objs[0].(*rbacv1.RoleBinding)
	require.True(t, ok)
	assert.Equal(t, "Role", rb1.RoleRef.Kind)
	assert.Equal(t, "editor", rb1.RoleRef.Name)

	// Second should be for ClusterRole in namespace
	rb2, ok := objs[1].(*rbacv1.RoleBinding)
	require.True(t, ok)
	assert.Equal(t, "ClusterRole", rb2.RoleRef.Kind)
	assert.Equal(t, "view", rb2.RoleRef.Name)
}

func TestOnKubeconfigChange_Nil(t *testing.T) {
	cfg := Config{}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	result, err := h.OnKubeconfigChange("key", nil)

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestOnKubeconfigChange_EnqueuesMatchingUserSyncGithub(t *testing.T) {
	cfg := Config{}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	// Add UserSyncGithub that matches the kubeconfig user
	kuserSyncGithub.AddUserSyncGithub(&klum.UserSyncGithub{
		ObjectMeta: metav1.ObjectMeta{Name: "sync-1"},
		Spec:       klum.UserSyncGithubSpec{User: "testuser"},
	})
	// Add one that doesn't match
	kuserSyncGithub.AddUserSyncGithub(&klum.UserSyncGithub{
		ObjectMeta: metav1.ObjectMeta{Name: "sync-2"},
		Spec:       klum.UserSyncGithubSpec{User: "otheruser"},
	})

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	kc := &klum.Kubeconfig{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
	}

	result, err := h.OnKubeconfigChange("testuser", kc)

	require.NoError(t, err)
	assert.Equal(t, kc, result)

	// Only sync-1 should be enqueued
	require.Len(t, kuserSyncGithub.EnqueuedIDs, 1)
	assert.Equal(t, "sync-1", kuserSyncGithub.EnqueuedIDs[0])
}

func TestUpdateUserDefaults_NoChangesNeeded(t *testing.T) {
	cfg := Config{
		ContextName: "test-context",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	// User already has both Context and ContextNamespace set
	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
		Spec: klum.UserSpec{
			Context:          "existing-context",
			ContextNamespace: "existing-namespace",
		},
	}

	err := h.updateUserDefaults(user)

	require.NoError(t, err)
	// User should not be updated since no changes were made
	assert.Equal(t, "existing-context", user.Spec.Context)
	assert.Equal(t, "existing-namespace", user.Spec.ContextNamespace)
}

func TestUpdateUserDefaults_SetsContext(t *testing.T) {
	cfg := Config{
		ContextName: "default-context",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	// User has no Context but has ContextNamespace
	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
		Spec: klum.UserSpec{
			Context:          "",
			ContextNamespace: "my-namespace",
		},
	}

	err := h.updateUserDefaults(user)

	require.NoError(t, err)
	// Context should be set to cfg.ContextName
	assert.Equal(t, "default-context", user.Spec.Context)
	// ContextNamespace should remain unchanged
	assert.Equal(t, "my-namespace", user.Spec.ContextNamespace)
}

func TestUpdateUserDefaults_SetsContextNamespace(t *testing.T) {
	cfg := Config{
		ContextName: "default-context",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	// User has Context but no ContextNamespace, with roles that have namespace
	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
		Spec: klum.UserSpec{
			Context:          "my-context",
			ContextNamespace: "",
			Roles: []klum.NamespaceRole{
				{Namespace: "dev-namespace", Role: "developer"},
			},
		},
	}

	err := h.updateUserDefaults(user)

	require.NoError(t, err)
	// Context should remain unchanged
	assert.Equal(t, "my-context", user.Spec.Context)
	// ContextNamespace should be set from first role namespace
	assert.Equal(t, "dev-namespace", user.Spec.ContextNamespace)
}

func TestUpdateUserDefaults_SetsBothFields(t *testing.T) {
	cfg := Config{
		ContextName: "cluster-context",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	// User has neither Context nor ContextNamespace, no roles
	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
		Spec:       klum.UserSpec{},
	}

	err := h.updateUserDefaults(user)

	require.NoError(t, err)
	// Context should be set to cfg.ContextName
	assert.Equal(t, "cluster-context", user.Spec.Context)
	// ContextNamespace should default to "default"
	assert.Equal(t, "default", user.Spec.ContextNamespace)
}
func TestOnSecretChange_NoAnnotation(t *testing.T) {
	cfg := Config{
		ContextName: "test-context",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()

	h := newTestHandler(cfg, kuser, kconfig, kuserSyncGithub, "25")

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
	}

	result, err := h.OnSecretChange("test-secret", secret)
	require.NoError(t, err)
	assert.Equal(t, secret, result)
}

func TestOnSecretChange_Valid(t *testing.T) {
	cfg := Config{
		ContextName: "test-context",
		Server:      "https://k8s.example.com",
		CA:          "test-ca-data",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()
	mockApply := NewMockApply()

	h := newTestHandlerWithApply(cfg, kuser, kconfig, kuserSyncGithub, mockApply, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
		Spec: klum.UserSpec{
			Context:          "user-context",
			ContextNamespace: "user-namespace",
		},
	}
	kuser.AddUser(user)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
			Annotations: map[string]string{
				"objectset.rio.cattle.io/id":         "klum-user",
				"objectset.rio.cattle.io/owner-name": "testuser",
			},
		},
		Type: v1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token": []byte("test-token"),
		},
	}

	_, err := h.OnSecretChange("test-secret", secret)
	require.NoError(t, err)

	// Verify ApplyObjects was called
	require.Len(t, mockApply.AppliedObjects, 1)
	kc, ok := mockApply.AppliedObjects[0].(*klum.Kubeconfig)
	require.True(t, ok)

	assert.Equal(t, "testuser", kc.Name)
	assert.Equal(t, "user-context", kc.Spec.CurrentContext)
	assert.Equal(t, "test-ca-data", kc.Spec.Clusters[0].Cluster.CertificateAuthorityData)
	assert.Equal(t, "test-token", kc.Spec.AuthInfos[0].AuthInfo.Token)
}

func TestOnSecretChange_UpdateDefaults(t *testing.T) {
	cfg := Config{
		ContextName: "default-context",
	}
	kuser := NewMockUserController()
	kconfig := NewMockKubeconfigController()
	kuserSyncGithub := NewMockUserSyncGithubController()
	mockApply := NewMockApply()

	h := newTestHandlerWithApply(cfg, kuser, kconfig, kuserSyncGithub, mockApply, "25")

	user := &klum.User{
		ObjectMeta: metav1.ObjectMeta{Name: "testuser"},
		Spec:       klum.UserSpec{}, // Empty spec, triggers defaults update
	}
	kuser.AddUser(user)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
			Annotations: map[string]string{
				"objectset.rio.cattle.io/id":         "klum-user",
				"objectset.rio.cattle.io/owner-name": "testuser",
			},
		},
		Type: v1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token":  []byte("test-token"),
			"ca.crt": []byte("ca-cert"),
		},
	}

	_, err := h.OnSecretChange("test-secret", secret)
	require.NoError(t, err)

	// Verify user was updated with defaults
	updatedUser, _ := kuser.Get("testuser", metav1.GetOptions{})
	assert.Equal(t, "default-context", updatedUser.Spec.Context)
	assert.Equal(t, "default", updatedUser.Spec.ContextNamespace)

	// Verify Apply was called with corrected defaults
	require.Len(t, mockApply.AppliedObjects, 1)
	kc, ok := mockApply.AppliedObjects[0].(*klum.Kubeconfig)
	require.True(t, ok)
	assert.Equal(t, "default-context", kc.Spec.CurrentContext)
}
