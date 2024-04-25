package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

type SubResult struct {
	// Requeue tells the Controller to requeue the reconcile key.  Defaults to false.
	Requeue bool

	// RequeueAfter if greater than 0, tells the Controller to requeue the reconcile key after the Duration.
	// Implies that Requeue is true, there is no need to set Requeue to true at the same time as RequeueAfter.
	RequeueAfter time.Duration

	// Return tells the Controller to return ctrl.Result.  Defaults to false.
	Return bool
}

func (r *SubResult) KubeResult() ctrl.Result {
	return ctrl.Result{Requeue: r.Requeue, RequeueAfter: r.RequeueAfter}
}
