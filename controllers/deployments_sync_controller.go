/*
Copyright 2021.

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

package controllers

import (
	"context"

	cachev1alpha1 "github.com/example/memcached-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	deploymentSyncLabelKey = "memcached-operator/associated-memcached-deployment-name"
)

// DeploymentSyncReconciler reconciles a Deployments object
type DeploymentSyncReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DeploymentSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("deploymentSync", req.NamespacedName)

	// fetch the deployment for this request
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, req.NamespacedName, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Deployment resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Check for a label on the deployment that indicates there's an associated
	// memcached deployment to scale.
	deploymentLabels := deployment.GetLabels()
	memcachedName, ok := deploymentLabels[deploymentSyncLabelKey]
	if !ok {
		// The deployment doesn't have the label so it must not have
		// an associated memcached. Log lines here might be made
		// available at a higher verbosity.
		// log.Info("No associated memcached deployment for deployment")
		return ctrl.Result{}, nil
	}

	deploymentReplicas := *deployment.Spec.Replicas

	memcachedKey := types.NamespacedName{Namespace: req.Namespace, Name: memcachedName}

	// Fetch the Memcached instance
	memcached := &cachev1alpha1.Memcached{}
	err = r.Get(ctx, memcachedKey, memcached)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Memcached resource not found. Nothing to do.")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Memcached")
		return ctrl.Result{}, err
	}

	if deploymentReplicas == memcached.Spec.Size {
		// the memcached size is the same as the deployment
		// so there's nothing to do
		// log.Info("Replica count on deployment does not differ from memcached")
		return ctrl.Result{}, nil
	}

	toPatch := client.MergeFrom(memcached.DeepCopy())
	memcached.Spec.Size = deploymentReplicas

	log.Info(
		"Replica count on deployment has changed. Syncing memcached",
		"memcached-identifier",
		memcachedKey.String(),
		"from", memcached.Spec.Size,
		"to", deploymentReplicas,
	)

	if err := r.Patch(ctx, memcached, toPatch); err != nil {
		// unsuccessful patch attempt, so return an error
		// and requeue.
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// We only watch for changes to deployment.
		For(&appsv1.Deployment{}).
		Complete(r)
}
