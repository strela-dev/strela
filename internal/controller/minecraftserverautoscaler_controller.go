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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"math"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streladevv1 "strela.dev/strela/api/v1"
)

// MinecraftServerAutoscalerReconciler reconciles a MinecraftServerAutoscaler object
type MinecraftServerAutoscalerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=strela.dev,resources=minecraftserverautoscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=strela.dev,resources=minecraftserverautoscalers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=strela.dev,resources=minecraftserverautoscalers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MinecraftServerAutoscaler object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *MinecraftServerAutoscalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var autoscaler streladevv1.MinecraftServerAutoscaler
	if err := r.Get(ctx, req.NamespacedName, &autoscaler); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, could have been deleted after reconcile request.
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch MinecraftServerAutoscaler")
		return ctrl.Result{}, err
	}

	// Evaluate autoscaler
	err := r.evaluateAutoscaler(ctx, &autoscaler)
	if err != nil {
		logger.Error(err, "failed to evaluate autoscaler", "autoscaler", autoscaler.Name)
		return ctrl.Result{}, err
	}

	// Schedule next evaluation
	duration := time.Duration(autoscaler.Spec.MinScalePause) * time.Second
	return ctrl.Result{RequeueAfter: duration}, nil
}

func (r *MinecraftServerAutoscalerReconciler) evaluateAutoscaler(ctx context.Context, autoscaler *streladevv1.MinecraftServerAutoscaler) error {
	// Fetch target MinecraftDeployment
	var targetDeployment streladevv1.MinecraftDeployment
	if err := r.Get(ctx, types.NamespacedName{Name: autoscaler.Spec.TargetDeployment, Namespace: autoscaler.Namespace}, &targetDeployment); err != nil {
		return err
	}

	// Determine desired replica count based on autoscaler type and target group status
	desiredReplicas, err := r.determineDesiredReplicaCount(ctx, autoscaler, &targetDeployment)
	if err != nil {
		return err
	}

	// Update MinecraftServerGroup if necessary
	if targetDeployment.Spec.Replicas != desiredReplicas {
		targetDeployment.Spec.Replicas = desiredReplicas
		if err := r.Update(ctx, &targetDeployment); err != nil {
			return err
		}
	}

	return nil
}

// getMinecraftServersForAutoscaler fetches all MinecraftServer instances associated with the given autoscaler.
func (r *MinecraftServerAutoscalerReconciler) getMinecraftServersForAutoscaler(ctx context.Context, autoscaler *streladevv1.MinecraftServerAutoscaler) ([]streladevv1.MinecraftServer, error) {
	var minecraftServerList streladevv1.MinecraftServerList

	// Define the label selector based on the autoscaler's target deployment
	labelSelector := labels.SelectorFromSet(labels.Set{"autoscalerTargetDeployment": autoscaler.Spec.TargetDeployment})

	// ListOptions to include the label selector
	listOpts := []client.ListOption{
		client.InNamespace(autoscaler.Namespace),
		client.MatchingLabelsSelector{Selector: labelSelector},
	}

	// Fetching the list of MinecraftServer resources
	if err := r.List(ctx, &minecraftServerList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list MinecraftServers for autoscaler %s: %w", autoscaler.Name, err)
	}

	// Now you can return minecraftServerList.Items, which is a slice of MinecraftServer
	return minecraftServerList.Items, nil
}

func (r *MinecraftServerAutoscalerReconciler) determineDesiredReplicaCount(ctx context.Context, autoscaler *streladevv1.MinecraftServerAutoscaler, targetGroup *streladevv1.MinecraftDeployment) (int, error) {
	switch autoscaler.Spec.Type {
	case streladevv1.ServerAutoscale:
		return r.determineReplicaCountForServer(ctx, autoscaler)
	case streladevv1.SlotAutoscale:
		return r.determineReplicaCountForSlots(ctx, autoscaler, targetGroup)
	default:
		// Handle unexpected autoscaler type
		return targetGroup.Spec.Replicas, nil
	}
}

func (r *MinecraftServerAutoscalerReconciler) determineReplicaCountForServer(ctx context.Context, autoscaler *streladevv1.MinecraftServerAutoscaler) (int, error) {
	// Placeholder: Implement logic to determine replica count based on server count
	servers, err := r.getMinecraftServersForAutoscaler(ctx, autoscaler)
	if err != nil {
		return 0, err
	}

	ingameServerCount := 0
	for _, server := range servers {
		if server.Status.Ingame {
			ingameServerCount++
		}
	}

	// Placeholder for invoking custom logic function
	// In Go, you would likely use a different approach to execute custom logic
	// For simplicity, let's assume a static logic for now
	return ingameServerCount + 1, nil // Example logic, adjust as necessary
}

func (r *MinecraftServerAutoscalerReconciler) determineReplicaCountForSlots(ctx context.Context, autoscaler *streladevv1.MinecraftServerAutoscaler, targetGroup *streladevv1.MinecraftDeployment) (int, error) {
	servers, err := r.getMinecraftServersForAutoscaler(ctx, autoscaler)
	if err != nil {
		return 0, err
	}

	if len(servers) == 0 {
		return 1, nil
	}

	maxPlayerCount := servers[0].Spec.MaxPlayers // Assuming all servers have the same MaxPlayers
	maxSlots := 0
	occupiedSlots := 0
	for _, server := range servers {
		maxSlots += server.Spec.MaxPlayers
		occupiedSlots += server.Status.PlayerCount
	}

	// Example static logic to determine additional slots needed
	// Replace this with actual logic possibly involving autoscaler.Spec.Function
	desiredAvailableSlots := 10 // Placeholder value
	missingSlots := desiredAvailableSlots - (maxSlots - occupiedSlots)
	replicaDiff := int(math.Ceil(float64(missingSlots) / float64(maxPlayerCount)))

	return targetGroup.Spec.Replicas + replicaDiff, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftServerAutoscalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&streladevv1.MinecraftServerAutoscaler{}).
		Complete(r)
}
