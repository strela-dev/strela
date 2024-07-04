package util

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const serviceAccountName = "strela-minecraft-server-sa"
const roleBindingName = "strela-minecraft-server-role-binding"

func EnsureServiceAccount(c client.Client, ctx context.Context, namespace string) (string, error) {
	sa := &corev1.ServiceAccount{}
	err := c.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: namespace}, sa)

	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return "", err
		}
		if err := CreateServiceAccount(c, ctx, namespace); err != nil {
			return "", err
		}
		if err := AddServiceAccountInNamespaceToClusterRoleBinding(c, ctx, namespace); err != nil {
			return "", err
		}
	}
	return serviceAccountName, nil
}

func CreateServiceAccount(c client.Client, ctx context.Context, namespace string) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	return c.Create(ctx, serviceAccount)
}

func AddServiceAccountInNamespaceToClusterRoleBinding(c client.Client, ctx context.Context, namespace string) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err := c.Get(ctx, types.NamespacedName{Name: roleBindingName}, clusterRoleBinding)

	if err != nil {
		if err := CreateClusterRoleBinding(c, ctx); err != nil {
			return err
		}
		// Fetch the newly created ClusterRoleBinding
		if err := c.Get(ctx, types.NamespacedName{Name: roleBindingName}, clusterRoleBinding); err != nil {
			return err
		}
	}

	clusterRoleBinding.Subjects = append(clusterRoleBinding.Subjects, rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      serviceAccountName,
		Namespace: namespace,
	})

	return c.Update(ctx, clusterRoleBinding)
}

func CreateClusterRoleBinding(c client.Client, ctx context.Context) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleBindingName,
		},
		Subjects: []rbacv1.Subject{},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "strela-minecraft-server-role",
		},
	}

	return c.Create(ctx, clusterRoleBinding)
}
