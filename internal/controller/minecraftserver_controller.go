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
	"errors"
	"github.com/strela-dev/strela/internal/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"

	streladevv1 "github.com/strela-dev/strela/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const minecraftServerFinalizer = "finalizer.strela.dev"

// MinecraftServerReconciler reconciles a MinecraftServer object
type MinecraftServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=strela.dev,resources=minecraftservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=strela.dev,resources=minecraftservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=strela.dev,resources=minecraftservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MinecraftServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *MinecraftServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconcile of ", "name", req.NamespacedName.String())

	var minecraftServer streladevv1.MinecraftServer
	if err := r.Get(ctx, req.NamespacedName, &minecraftServer); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var currentPod corev1.Pod
	podNamespacedName := types.NamespacedName{Namespace: minecraftServer.Namespace, Name: minecraftServer.Name}
	errPod := r.Get(ctx, podNamespacedName, &currentPod)

	res, err := r.handleFinalizer(ctx, &minecraftServer, currentPod)
	if err != nil || res != nil {
		return *res, err
	}

	if errPod == nil {
		//Pod already exist

		if currentPod.Status.Phase != corev1.PodRunning {
			return ctrl.Result{}, nil
		}

		allContainersReady := true
		for _, containerStatus := range currentPod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				allContainersReady = false
			}
		}

		if allContainersReady {
			if minecraftServer.Status.Ready {
				return ctrl.Result{}, nil
			}
			minecraftServer.Status.Ready = true
			minecraftServer.Status.ReadyTime = metav1.Now()
			if err := r.Status().Update(ctx, &minecraftServer); err != nil {
				log.Error(err, "unable to update status", minecraftServer)
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	if !apierrors.IsNotFound(errPod) {
		return ctrl.Result{}, client.IgnoreNotFound(errPod)
	}

	newPod, err := createNewPodFromTemplate(ctx, r, minecraftServer)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := ctrl.SetControllerReference(&minecraftServer, newPod, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, newPod); err != nil {
		log.Error(err, "unable to create Pod for MinecraftServer", "pod", minecraftServer)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MinecraftServerReconciler) handleFinalizer(ctx context.Context, minecraftServer *streladevv1.MinecraftServer, currPod corev1.Pod) (*ctrl.Result, error) {
	if minecraftServer.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(minecraftServer, minecraftServerFinalizer) {
			controllerutil.AddFinalizer(minecraftServer, minecraftServerFinalizer)
			if err := r.Update(ctx, minecraftServer); err != nil {
				return &ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(minecraftServer, minecraftServerFinalizer) {
			// our finalizer is present, so lets handle any external dependency

			var podExists = currPod.Name != ""
			if podExists && currPod.DeletionTimestamp != nil {
				// Pod is being deleted
				return &ctrl.Result{}, nil
			}

			if podExists {
				if err := r.Delete(ctx, &currPod); err != nil {
					return &ctrl.Result{}, err
				}
				return &ctrl.Result{}, nil
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(minecraftServer, minecraftServerFinalizer)
			if err := r.Update(ctx, minecraftServer); err != nil {
				return &ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return &ctrl.Result{}, nil
	}
	return nil, nil
}

func createNewPodFromTemplate(ctx context.Context, r *MinecraftServerReconciler, minecraftServer streladevv1.MinecraftServer) (*corev1.Pod, error) {
	//log := log.FromContext(ctx)
	minecraftServerContainer, err := findMinecraftServerContainer(&minecraftServer)
	if err != nil {
		return nil, err
	}

	println("Found container with name " + minecraftServerContainer.Name)

	//namespaceName, err := r.findNamespaceName(ctx)
	//if err != nil {
	//	log.Error(err, "unable to find strela namespace", minecraftServer)
	//}

	podTemplate := minecraftServer.Spec.Template
	podName := minecraftServer.Name
	objectMeta := *podTemplate.ObjectMeta.DeepCopy()
	objectMeta.Labels = minecraftServer.Labels
	pod := &corev1.Pod{
		ObjectMeta: objectMeta,
		Spec:       *podTemplate.Spec.DeepCopy(),
	}

	pod.ObjectMeta.Name = podName
	pod.ObjectMeta.Namespace = minecraftServer.Namespace

	if minecraftServer.Spec.ConfigDir != "" {
		configureInitContainer(pod, minecraftServerContainer, &minecraftServer)
	}

	serviceAccountName, err := r.findServiceAccountName(ctx, minecraftServer.Namespace)
	if err == nil {
		pod.Spec.ServiceAccountName = serviceAccountName
	}

	containers := make([]corev1.Container, 0, 1+len(pod.Spec.Containers))
	containers = append(containers, createSideCarContainer(minecraftServer.Spec.Type, podName))
	containers = append(containers, pod.Spec.Containers...)
	pod.Spec.Containers = containers

	return pod, nil
}

func configureInitContainer(pod *corev1.Pod, minecraftServerContainer *corev1.Container, minecraftServer *streladevv1.MinecraftServer) {
	foundVolumeName := findVolumeName(minecraftServerContainer, minecraftServer.Spec.ConfigDir)
	if foundVolumeName == "" {
		foundVolumeName = "data"
		addEmptyDirVolume(pod, minecraftServerContainer, minecraftServer.Spec.ConfigDir, foundVolumeName)
	}
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, createInitContainer(minecraftServer, foundVolumeName))
}

func addEmptyDirVolume(pod *corev1.Pod, minecraftContainer *corev1.Container, mountPath string, volumeName string) {
	newVolume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, newVolume)

	volumeMount := corev1.VolumeMount{Name: volumeName, MountPath: mountPath}

	minecraftContainer.VolumeMounts = append(minecraftContainer.VolumeMounts, volumeMount)
}

func findVolumeName(container *corev1.Container, configDir string) string {
	for _, volumeMount := range container.VolumeMounts {
		if volumeMount.MountPath == configDir {
			println("for " + volumeMount.MountPath + " " + configDir)
			return volumeMount.Name
		}
	}
	return ""
}

func createInitContainer(server *streladevv1.MinecraftServer, volumeName string) corev1.Container {
	configDir := server.Spec.ConfigDir
	if configDir == "" {
		configDir = "/data"
	}

	sidecar := corev1.Container{
		Name:  "strela-init-container",
		Image: "ghcr.io/strela-dev/minecraft-configurator:main",
		Env: []corev1.EnvVar{
			{
				Name:  "MODE",
				Value: string(server.Spec.ConfigurationMode),
			},
			{
				Name:  "MAX_PLAYERS",
				Value: strconv.Itoa(server.Spec.MaxPlayers),
			},
			{
				Name:  "FORWARDING_SECRET",
				Value: "abcde",
			},
			{
				Name:  "DATA_DIR",
				Value: configDir,
			},
		},
		ImagePullPolicy: corev1.PullAlways,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeName,
				MountPath: configDir,
			},
		},
	}
	return sidecar
}

func createSideCarContainer(minecraftServerType streladevv1.MinecraftServerType, podName string) corev1.Container {
	var envs []corev1.EnvVar
	envs = append(envs, corev1.EnvVar{Name: "POD_NAME", Value: podName})

	//if minecraftServerType == streladevv1.Proxy {
	//	envs = append(envs, corev1.EnvVar{Name: "PROXY_PROTO", Value: "2"})
	//}

	sidecar := corev1.Container{
		Name:            "strela-sidecar",
		Image:           "ghcr.io/strela-dev/strela-sidecar:main",
		Env:             envs,
		ImagePullPolicy: corev1.PullAlways,
	}
	return sidecar
}

func findMinecraftServerContainer(server *streladevv1.MinecraftServer) (*corev1.Container, error) {
	expectedMcContainerName := server.Spec.Container
	containers := server.Spec.Template.Spec.Containers
	if len(expectedMcContainerName) == 0 {
		if len(containers) != 1 {
			return nil, errors.New("more then one container and 'container' field is empty. Set the 'container' field to the name of your container running the minecraft server")
		}
		return &containers[0], nil
	}

	for _, container := range containers {
		if container.Name == expectedMcContainerName {
			return &container, nil
		}
	}
	return nil, errors.New("'container' field was set but a container with the name was not found")
}

func (r *MinecraftServerReconciler) findServiceAccountName(ctx context.Context, namespace string) (string, error) {
	return util.EnsureServiceAccount(r.Client, ctx, namespace)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&streladevv1.MinecraftServer{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
