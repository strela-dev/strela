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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streladevv1 "strela.dev/strela/api/v1"
)

// MinecraftServerSetReconciler reconciles a MinecraftServerSet object
type MinecraftServerSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=strela.dev,resources=minecraftserversets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=strela.dev,resources=minecraftserversets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=strela.dev,resources=minecraftserversets/finalizers,verbs=update

//+kubebuilder:rbac:groups=strela.dev,resources=minecraftservers,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MinecraftServerSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *MinecraftServerSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconcile of " + req.NamespacedName.String())

	var minecraftServerSet streladevv1.MinecraftServerSet
	if err := r.Get(ctx, req.NamespacedName, &minecraftServerSet); err != nil {
		/// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Construct the owner reference

	// List options with matching fields
	var childMinecraftServers streladevv1.MinecraftServerList
	if err := r.List(ctx, &childMinecraftServers, client.InNamespace(req.Namespace), client.MatchingFields{minecraftServerOwnerKey: req.Name}); err != nil {
		logger.Error(err, "unable to list child MinecraftServers")
		return ctrl.Result{}, err
	}

	minecraftServers := childMinecraftServers.Items
	ingameCount := determineIngameServerCount(minecraftServers)
	readyCount := determineReadyServerCount(minecraftServers)
	notIngameCount := len(minecraftServers) - ingameCount

	if notIngameCount == minecraftServerSet.Spec.Replicas {
		//update status
		needUpdate := false
		if minecraftServerSet.Status.Replicas != len(minecraftServers) {
			minecraftServerSet.Status.Replicas = len(minecraftServers)
			needUpdate = true
		}
		if minecraftServerSet.Status.Ready != readyCount {
			minecraftServerSet.Status.Ready = readyCount
			needUpdate = true
		}
		if minecraftServerSet.Status.Ingame != ingameCount {
			minecraftServerSet.Status.Ingame = ingameCount
			needUpdate = true
		}

		if needUpdate {
			if err := r.Status().Update(ctx, &minecraftServerSet); err != nil {
				logger.Error(err, "failed to update MinecraftDeployment status")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	if notIngameCount > minecraftServerSet.Spec.Replicas {
		serverToStop := minecraftServers[0]

		if err := r.Delete(ctx, &serverToStop); err != nil {
			logger.Error(err, "unable to create Pod for MinecraftServer", "minecraftServer", serverToStop)
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	newMinecraftServer := createNewMinecraftServerFromTemplate(minecraftServerSet)

	if err := ctrl.SetControllerReference(&minecraftServerSet, newMinecraftServer, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, newMinecraftServer); err != nil {
		logger.Error(err, "unable to create Pod for MinecraftServer", "minecraftServer", newMinecraftServer)
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

func createNewMinecraftServerFromTemplate(set streladevv1.MinecraftServerSet) *streladevv1.MinecraftServer {
	template := set.Spec.Template
	objectMeta := *template.ObjectMeta.DeepCopy()
	objectMeta.Labels = set.Labels
	server := &streladevv1.MinecraftServer{
		ObjectMeta: objectMeta,
		Spec:       *template.Spec.DeepCopy(),
	}

	server.ObjectMeta.Name = ""
	server.ObjectMeta.GenerateName = set.Name + "-"
	server.ObjectMeta.Namespace = set.Namespace

	return server
}

func determineIngameServerCount(servers []streladevv1.MinecraftServer) int {
	count := 0
	for _, server := range servers {
		if server.Status.Ingame {
			count++
		}
	}
	return count
}

func determineReadyServerCount(servers []streladevv1.MinecraftServer) int {
	count := 0
	for _, server := range servers {
		if server.Status.Ready {
			count++
		}
	}
	return count
}

var (
	minecraftServerOwnerKey = ".metadata.controller"
	apiGVStr                = streladevv1.GroupVersion.String()
)

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftServerSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &streladevv1.MinecraftServer{}, minecraftServerOwnerKey, func(rawObj client.Object) []string {

		minecraftServer := rawObj.(*streladevv1.MinecraftServer)
		owner := metav1.GetControllerOf(minecraftServer)
		if owner == nil {
			return nil
		}

		if owner.APIVersion != apiGVStr || owner.Kind != "MinecraftServerSet" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&streladevv1.MinecraftServerSet{}).
		Owns(&streladevv1.MinecraftServer{}).
		Complete(r)
}
