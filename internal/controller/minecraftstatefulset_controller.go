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
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streladevv1 "github.com/strela-dev/strela/api/v1"
)

// MinecraftStatefulSetReconciler reconciles a MinecraftStatefulSet object
type MinecraftStatefulSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=strela.dev,resources=minecraftstatefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=strela.dev,resources=minecraftstatefulsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=strela.dev,resources=minecraftstatefulsets/finalizers,verbs=update

//+kubebuilder:rbac:groups=strela.dev,resources=minecraftservers,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MinecraftStatefulSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *MinecraftStatefulSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconcile of " + req.NamespacedName.String())

	var statefulset streladevv1.MinecraftStatefulSet
	if err := r.Get(ctx, req.NamespacedName, &statefulset); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var childMinecraftServers streladevv1.MinecraftServerList
	if err := r.List(ctx, &childMinecraftServers, client.InNamespace(req.Namespace), client.MatchingFields{minecraftServerOwnerKeyForStatefulSet: req.Name}); err != nil {
		logger.Error(err, "unable to list child MinecraftServers")
		return ctrl.Result{}, err
	}

	//childMinecraftServers.Items sorted by name
	var sortedMinecraftServers = childMinecraftServers.Items
	sort.Slice(sortedMinecraftServers, func(i, j int) bool {
		return sortedMinecraftServers[i].Name < sortedMinecraftServers[j].Name
	})

	res, err := r.handleStatusUpdates(ctx, &statefulset, sortedMinecraftServers, logger)
	if err != nil || res != nil {
		return *res, err
	}

	msTemplateHash, err := statefulset.Spec.Template.GenerateTemplateSpecHash()
	if err != nil {
		logger.Error(err, "failed to generate PodTemplateSpec hash")
		return ctrl.Result{}, err
	}

	replicas := statefulset.Spec.Replicas
	currReplicas := len(sortedMinecraftServers)
	//get latest from sortedMinecraftServers
	var latestMinecraftServer *streladevv1.MinecraftServer = nil
	if currReplicas > 0 {
		latestMinecraftServer = &sortedMinecraftServers[currReplicas-1]
	}
	//check if one of the servers is not ready
	for _, server := range sortedMinecraftServers {
		if !server.Status.Ready {
			return ctrl.Result{}, nil
		}
		//if server is not ready longer then minReadySeconds
		readyPlusMinReadySeconds := server.Status.ReadyTime.Add(time.Duration(statefulset.Spec.MinReadySeconds) * time.Second)
		if readyPlusMinReadySeconds.After(time.Now()) {
			return ctrl.Result{RequeueAfter: time.Until(readyPlusMinReadySeconds)}, nil
		}
	}

	if currReplicas < replicas {
		//start a new MinecraftServer

		var canStartNewServer = latestMinecraftServer == nil || latestMinecraftServer.Status.Ready
		if canStartNewServer {
			newMinecraftServer, err := r.newMinecraftServerForStatefulSet(ctx, &statefulset, sortedMinecraftServers, msTemplateHash, logger)
			if err != nil {
				logger.Error(err, "unable to create MinecraftServer")
				return ctrl.Result{}, err
			}

			if err := ctrl.SetControllerReference(&statefulset, newMinecraftServer, r.Scheme); err != nil {
				logger.Error(err, "unable to set controller reference", "MinecraftServer", newMinecraftServer)
				return ctrl.Result{}, err
			}

			if err := r.Create(ctx, newMinecraftServer); err != nil {
				logger.Error(err, "unable to create MinecraftServer", "MinecraftServer", newMinecraftServer)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if currReplicas > replicas && latestMinecraftServer != nil {

		//if latestMinecraftServer terminating return
		if latestMinecraftServer.DeletionTimestamp != nil {
			return ctrl.Result{}, nil
		}

		//delete latestMinecraftServer
		if err := r.Delete(ctx, latestMinecraftServer); err != nil {
			logger.Error(err, "unable to delete MinecraftServer", "MinecraftServer", latestMinecraftServer)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	result, err := r.handleStatefulSetUpdate(ctx, sortedMinecraftServers, msTemplateHash, logger)
	if err != nil || result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

func (r *MinecraftStatefulSetReconciler) handleStatefulSetUpdate(
	ctx context.Context,
	sortedMineCraftServers []streladevv1.MinecraftServer,
	msTemplateHash string,
	logger logr.Logger,
) (*ctrl.Result, error) {

	var lastServerNotMatchingHash *streladevv1.MinecraftServer = nil
	for i := len(sortedMineCraftServers) - 1; i >= 0; i-- {
		server := sortedMineCraftServers[i]
		if server.ObjectMeta.Labels["ms-template-hash"] != msTemplateHash {
			lastServerNotMatchingHash = &server
			break
		}
	}

	//delete lastServerNotMatchingHash if it is not nil and not being deleted
	if lastServerNotMatchingHash != nil && lastServerNotMatchingHash.DeletionTimestamp == nil {
		if err := r.Delete(ctx, lastServerNotMatchingHash); err != nil {
			logger.Error(err, "unable to delete MinecraftServer", "MinecraftServer", lastServerNotMatchingHash)
			return &ctrl.Result{}, err
		}
	}

	return nil, nil
}

func (r *MinecraftStatefulSetReconciler) newMinecraftServerForStatefulSet(
	ctx context.Context,
	statefulset *streladevv1.MinecraftStatefulSet,
	sortedMineCraftServers []streladevv1.MinecraftServer,
	msTemplateHash string,
	logger logr.Logger,
) (*streladevv1.MinecraftServer, error) {

	template := statefulset.Spec.Template
	objectMeta := *template.ObjectMeta.DeepCopy()
	objectMeta.Labels = statefulset.Labels
	objectMeta.Labels["ms-template-hash"] = msTemplateHash

	newNumber := r.determineNumberForNewMinecraftServer(statefulset.Name, sortedMineCraftServers)
	objectMeta.Name = statefulset.Name + "-" + strconv.Itoa(newNumber)
	objectMeta.Namespace = statefulset.Namespace

	objectMeta.Labels["numerical-id"] = strconv.Itoa(newNumber)
	objectMeta.Labels["server-name"] = objectMeta.Name

	spec := *template.Spec.DeepCopy()
	server := &streladevv1.MinecraftServer{
		ObjectMeta: objectMeta,
		Spec:       spec,
		Status: streladevv1.MinecraftServerStatus{
			Ready:       false,
			Ingame:      false,
			PlayerCount: 0,
		},
	}

	err := r.setupPersistentVolumes(ctx, statefulset, server)
	if err != nil {
		return nil, err
	}

	return server, nil
}

func (r *MinecraftStatefulSetReconciler) setupPersistentVolumes(
	ctx context.Context,
	statefulset *streladevv1.MinecraftStatefulSet,
	minecraftServer *streladevv1.MinecraftServer,
) error {
	for _, pvc := range statefulset.Spec.VolumeClaimTemplates {
		err := r.setupSinglePersistentVolumeClaim(ctx, statefulset, minecraftServer, pvc)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *MinecraftStatefulSetReconciler) setupSinglePersistentVolumeClaim(
	ctx context.Context,
	statefulset *streladevv1.MinecraftStatefulSet,
	minecraftServer *streladevv1.MinecraftServer,
	pvcTemplate streladevv1.SelfVolumeClaimSpec,
) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcTemplate.Metadata.Name,
			Namespace: statefulset.Namespace,
		},
		Spec: *pvcTemplate.Spec.DeepCopy(),
	}
	pvc.Name = pvc.Name + "-" + minecraftServer.Name
	pvc.Labels = statefulset.Labels

	if err := r.ensurePvcExists(ctx, pvc); err != nil {
		return err
	}

	podSpec := &minecraftServer.Spec.Template.Spec

	//add volume to pod spec
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: pvcTemplate.Metadata.Name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc.Name,
			},
		},
	})

	return nil
}

func (r *MinecraftStatefulSetReconciler) ensurePvcExists(
	ctx context.Context,
	pvc *corev1.PersistentVolumeClaim,
) error {
	err := r.Get(ctx, client.ObjectKey{Namespace: pvc.Namespace, Name: pvc.Name}, pvc)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Log.Error(err, "failed to get PVC")
			return err
		}
		//create pvc
		err = r.Create(ctx, pvc)
		if err != nil {
			log.Log.Error(err, "failed to create PVC")
			return err
		}
	}
	return nil
}

func (r *MinecraftStatefulSetReconciler) determineNumberForNewMinecraftServer(
	namePrefix string,
	sortedMineCraftServers []streladevv1.MinecraftServer,
) int {
	var startNumber = 1

	var newName = namePrefix + "-" + strconv.Itoa(startNumber)
	for r.isNameInUse(sortedMineCraftServers, newName) {
		startNumber++
		newName = namePrefix + "-" + strconv.Itoa(startNumber)
	}
	return startNumber
}

func (r *MinecraftStatefulSetReconciler) isNameInUse(servers []streladevv1.MinecraftServer, name string) bool {
	for _, server := range servers {
		if strings.EqualFold(server.Name, name) {
			return true
		}
	}
	return false
}

func (r *MinecraftStatefulSetReconciler) handleStatusUpdates(
	ctx context.Context,
	statefulset *streladevv1.MinecraftStatefulSet,
	servers []streladevv1.MinecraftServer,
	logger logr.Logger,
) (*ctrl.Result, error) {
	oldStatus := statefulset.Status.DeepCopy()
	// Update the MinecraftStatefulSet status
	statefulset.Status.Replicas = len(servers)
	statefulset.Status.Ready = r.determineReadyCount(servers)
	if !r.areStatusesEqual(*oldStatus, statefulset.Status) {
		if err := r.Status().Update(ctx, statefulset); err != nil {
			logger.Error(err, "failed to update MinecraftStatefulSet status")
			return &ctrl.Result{}, err
		}
		return &ctrl.Result{Requeue: true}, nil
	}
	return nil, nil
}

func (r *MinecraftStatefulSetReconciler) areStatusesEqual(a, b streladevv1.MinecraftStatefulSetStatus) bool {
	return a.Replicas == b.Replicas && a.Ready == b.Ready
}

func (r *MinecraftStatefulSetReconciler) determineReadyCount(servers []streladevv1.MinecraftServer) int {
	var readyCount = 0
	for _, server := range servers {
		if server.Status.Ready {
			readyCount++
		}
	}
	return readyCount
}

const (
	minecraftServerOwnerKeyForStatefulSet = ".metadata.controllerss"
)

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftStatefulSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &streladevv1.MinecraftServer{}, minecraftServerOwnerKeyForStatefulSet, func(rawObj client.Object) []string {

		minecraftServer := rawObj.(*streladevv1.MinecraftServer)
		owner := metav1.GetControllerOf(minecraftServer)
		if owner == nil {
			return nil
		}

		if owner.APIVersion != apiGVStr || owner.Kind != "MinecraftStatefulSet" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&streladevv1.MinecraftStatefulSet{}).
		Owns(&streladevv1.MinecraftServer{}).
		Complete(r)
}
