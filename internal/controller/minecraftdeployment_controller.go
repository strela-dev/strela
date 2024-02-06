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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Generate hash for the PodTemplateSpec
	podTemplateHash, err := deployment.Spec.Template.GenerateTemplateSpecHash()
	if err != nil {
		logger.Error(err, "failed to generate PodTemplateSpec hash")
		return ctrl.Result{}, err
	}
	hashedName := fmt.Sprintf("%s-%s", deployment.Name, podTemplateHash[:8])

	// List all MinecraftServerSets owned by this MinecraftDeployment
	serverSets, err := deployment.GetMinecraftSets(r.Client, ctx)
	if err != nil {
		logger.Error(err, "unable to list MinecraftServerSets")
		return ctrl.Result{}, err
	}

	// Determine the current ServerSet and outdated ServerSets
	var currentServerSet *streladevv1.MinecraftServerSet
	outdatedServerSets := make([]streladevv1.MinecraftServerSet, 0)
	for _, ss := range serverSets {
		if ss.Name == hashedName {
			currentServerSet = &ss
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
				Labels:    deployment.Labels,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(&deployment, streladevv1.GroupVersion.WithKind("MinecraftDeployment")),
				},
			},
			Spec: streladevv1.MinecraftServerSetSpec{
				Replicas: deployment.Spec.Replicas,
				Template: *deployment.Spec.Template.DeepCopy(),
			},
		}
		if err := r.Create(ctx, newServerSet); err != nil {
			logger.Error(err, "failed to create MinecraftServerSet", "MinecraftServerSet", newServerSet.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if currentServerSet.Spec.Replicas != deployment.Spec.Replicas {
		currentServerSet.Spec.Replicas = deployment.Spec.Replicas
		if err := r.Update(ctx, currentServerSet); err != nil {
			logger.Error(err, "failed to update MinecraftServerSet spec")
			return ctrl.Result{}, err
		}
	}

	// Update the MinecraftDeployment status
	deployment.Status.Replicas = currentServerSet.Status.Replicas
	deployment.Status.Ready = currentServerSet.Status.Ready
	deployment.Status.Ingame = currentServerSet.Status.Ingame
	if err := r.Status().Update(ctx, &deployment); err != nil {
		logger.Error(err, "failed to update MinecraftDeployment status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &streladevv1.MinecraftServerSet{}, streladevv1.MinecraftServerSetOwnerKey, func(rawObj client.Object) []string {

		minecraftServerSet := rawObj.(*streladevv1.MinecraftServerSet)
		owner := metav1.GetControllerOf(minecraftServerSet)
		if owner == nil {
			return nil
		}

		if owner.APIVersion != apiGVStr || owner.Kind != "MinecraftDeployment" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&streladevv1.MinecraftDeployment{}).
		Owns(&streladevv1.MinecraftServerSet{}).
		Complete(r)
}
