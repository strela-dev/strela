/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"hash/fnv"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streladevv1 "strela.dev/strela/api/v1"
)

// MinecraftDeploymentReconciler reconciles a MinecraftDeployment object
type MinecraftDeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=strela.dev,resources=minecraftdeployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=strela.dev,resources=minecraftdeployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=strela.dev,resources=minecraftdeployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MinecraftDeployment object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *MinecraftDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the MinecraftDeployment instance
	var deployment streladevv1.MinecraftDeployment
	if err := r.Get(ctx, req.NamespacedName, &deployment); err != nil {
		logger.Error(err, "unable to fetch MinecraftDeployment")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Generate hash for the PodTemplateSpec
	podTemplateHash, err := generatePodTemplateSpecHash(deployment.Spec.Template)
	if err != nil {
		logger.Error(err, "failed to generate PodTemplateSpec hash")
		return ctrl.Result{}, err
	}
	hashedName := fmt.Sprintf("%s-%s", deployment.Name, podTemplateHash[:10])

	// List all MinecraftServerSets owned by this MinecraftDeployment
	var serverSets streladevv1.MinecraftServerSetList
	if err := r.List(ctx, &serverSets, client.InNamespace(deployment.Namespace), client.MatchingFields{"metadata.name": hashedName}); err != nil {
		logger.Error(err, "unable to list child MinecraftServerSets")
		return ctrl.Result{}, err
	}

	// Determine the current ServerSet and outdated ServerSets
	var currentServerSet *streladevv1.MinecraftServerSet
	outdatedServerSets := make([]streladevv1.MinecraftServerSet, 0)
	for i, ss := range serverSets.Items {
		if ss.Name == hashedName {
			currentServerSet = &serverSets.Items[i]
		} else {
			outdatedServerSets = append(outdatedServerSets, ss)
		}
	}

	// Update replicas for outdated ServerSets to 0 and delete if necessary
	for _, ss := range outdatedServerSets {
		if ss.Spec.Replicas != 0 {
			ss.Spec.Replicas = 0
			if err := r.Update(ctx, &ss); err != nil {
				logger.Error(err, "failed to update outdated MinecraftServerSet replicas to 0", "MinecraftServerSet", ss.Name)
				return ctrl.Result{}, err
			}
		}

		// Delete ServerSet if it has no active servers
		if ss.Status.Replicas == 0 {
			if err := r.Delete(ctx, &ss); err != nil {
				logger.Error(err, "failed to delete outdated MinecraftServerSet", "MinecraftServerSet", ss.Name)
				return ctrl.Result{}, err
			}
		}
	}

	// If there is no current ServerSet, create one
	if currentServerSet == nil {
		newServerSet := &streladevv1.MinecraftServerSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hashedName,
				Namespace: deployment.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(&deployment, streladevv1.GroupVersion.WithKind("MinecraftDeployment")),
				},
			},
			Spec: streladevv1.MinecraftServerSetSpec{
				Replicas: deployment.Spec.Replicas,
				Template: deployment.Spec.Template,
			},
		}
		if err := r.Create(ctx, newServerSet); err != nil {
			logger.Error(err, "failed to create MinecraftServerSet", "MinecraftServerSet", newServerSet.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Update the MinecraftDeployment status
	deployment.Status.Replicas = currentServerSet.Status.Replicas
	deployment.Status.Ready = currentServerSet.Status.Ready
	if err := r.Status().Update(ctx, &deployment); err != nil {
		logger.Error(err, "failed to update MinecraftDeployment status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func generatePodTemplateSpecHash(template streladevv1.MinecraftServerTemplateSpec) (string, error) {
	hasher := fnv.New32a()
	if _, err := hasher.Write([]byte(fmt.Sprintf("%+v", template))); err != nil {
		return "", err
	}
	return strconv.FormatUint(uint64(hasher.Sum32()), 16), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&streladevv1.MinecraftDeployment{}).
		Owns(&streladevv1.MinecraftServerSet{}).
		Complete(r)
}
