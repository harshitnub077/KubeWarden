package main

import (
	"context"
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Permission represents a single verb on a single resource.
type Permission struct {
	Resource  string `json:"resource"`
	Verb      string `json:"verb"`
	Namespace string `json:"namespace"`
	Allowed   bool   `json:"allowed"`
}

// PermissionMatrix is the full set of permissions for a user.
type PermissionMatrix struct {
	User        string       `json:"user"`
	Permissions []Permission `json:"permissions"`
}

// The resources and verbs we will check.
var resourcesToCheck = []struct {
	Group    string
	Resource string
}{
	{"", "pods"},
	{"", "services"},
	{"", "secrets"},
	{"apps", "deployments"},
	{"resource.k8s.io", "resourceclaims"},
	{"resource.k8s.io", "deviceclasses"},
	{"resource.k8s.io", "resourceslices"},
}

var verbs = []string{"get", "list", "create", "delete"}

// buildPermissionMatrix evaluates RBAC for the given user across
// the key DRA-related resources.
func buildPermissionMatrix(config *rest.Config, user string, groups []string) (*PermissionMatrix, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}

	matrix := &PermissionMatrix{User: user}

	for _, res := range resourcesToCheck {
		for _, verb := range verbs {
			// Create a SubjectAccessReview — this is how Kubernetes evaluates RBAC
			sar := &authorizationv1.SubjectAccessReview{
				Spec: authorizationv1.SubjectAccessReviewSpec{
					User:   user,
					Groups: groups,
					ResourceAttributes: &authorizationv1.ResourceAttributes{
						Namespace: "default",
						Verb:      verb,
						Group:     res.Group,
						Resource:  res.Resource,
					},
				},
			}

			result, err := clientset.AuthorizationV1().
				SubjectAccessReviews().
				Create(context.TODO(), sar, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("SAR for %s/%s: %w", res.Resource, verb, err)
			}

			matrix.Permissions = append(matrix.Permissions, Permission{
				Resource:  res.Resource,
				Verb:      verb,
				Namespace: "default",
				Allowed:   result.Status.Allowed,
			})
		}
	}
	return matrix, nil
}
