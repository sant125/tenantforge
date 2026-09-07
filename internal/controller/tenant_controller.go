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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	IngressSourceLabelKey   string
	IngressSourceLabelValue string
}

// +kubebuilder:rbac:groups=multitenancy.tenantforge.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=multitenancy.tenantforge.io,resources=tenants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=multitenancy.tenantforge.io,resources=tenants/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete

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

	if tenant.Spec.NetworkIsolation == multitenancyv1alpha1.NetworkIsolationLevelIsolated {
		log.Info("Tenant requires network isolation, creating NetworkPolicy", "Tenant", tenant.Name)
		netpol := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tenant.Name + "-network-policy",
				Namespace: tenantNamespace,
			},
		}

		udpPort := intstr.FromInt(53)
		dnsEgress := []networkingv1.NetworkPolicyEgressRule{
			{
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: func() *corev1.Protocol {
							proto := corev1.ProtocolUDP
							return &proto
						}(),
						Port: &udpPort,
					},
				},
				To: []networkingv1.NetworkPolicyPeer{
					{
						IPBlock: &networkingv1.IPBlock{
							CIDR: "0.0.0.0/0",
						},
					},
				},
			},
		}
		ingressRules := []networkingv1.NetworkPolicyIngressRule{
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								r.IngressSourceLabelKey: r.IngressSourceLabelValue,
							},
						},
					},
				},
			},
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{},
					},
				},
			},
		}

		result, err = controllerutil.CreateOrUpdate(ctx, r.Client, netpol, func() error {
			netpol.Spec = networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
			}
			netpol.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{ingressRules[0], ingressRules[1]} // Allow ingress from the same namespace and from the ingress source
			netpol.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{dnsEgress[0]}                       // Allow only DNS egress
			netpol.Spec.PolicyTypes = []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			}
			return controllerutil.SetControllerReference(&tenant, netpol, r.Scheme)
		})
		if err != nil {
			log.Error(err, "Failed to create or update NetworkPolicy", "NetworkPolicy", netpol.Name)
			return ctrl.Result{}, err
		}

	} else {
		log.Info("Tenant does not require network isolation, skipping NetworkPolicy creation", "Tenant", tenant.Name)
		return ctrl.Result{}, nil
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
