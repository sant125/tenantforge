/*
Copyright 2026.

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

	multitenancyv1alpha1 "github.com/sant125/tenantforge/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=multitenancy.tenantforge.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=multitenancy.tenantforge.io,resources=tenants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=multitenancy.tenantforge.io,resources=tenants/finalizers,verbs=update

// Reconcile garante que existe um Namespace pra cada Tenant, com OwnerReference
// apontando pro Tenant — deletar o Tenant derruba o Namespace (e tudo dentro dele) em cascata.
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("Reconciling Tenant", "Name", req.NamespacedName)

	var tenant multitenancyv1alpha1.Tenant
	if err := r.Get(ctx, req.NamespacedName, &tenant); err != nil {
		log.Error(err, "Failed to get Tenant")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	tenantNamespace := tenant.Name

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tenantNamespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
		ns.Labels = map[string]string{
			"tenantforge.io/clientName": tenant.Spec.ClientName,
			"istio-injection":           "enabled",
		}
		return controllerutil.SetControllerReference(&tenant, ns, r.Scheme)
	})
	if err != nil {
		log.Error(err, "Failed to create or update Namespace", "Namespace", tenantNamespace)
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled Namespace", "Namespace", tenantNamespace, "Result", result)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&multitenancyv1alpha1.Tenant{}).
		Owns(&corev1.Namespace{}).
		Named("tenant").
		Complete(r)
}
