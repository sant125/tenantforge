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

	// minha API customizada, pra poder criar e trabalhar com Tenants
	multitenancyv1alpha1 "github.com/sant125/tenantforge/api/v1alpha1"
	"go.yaml.in/yaml/v2"

	// corev1, pra poder criar e trabalhar com Namespaces/Pods/Services/ConfigMaps/Secrets, etc.
	corev1 "k8s.io/api/core/v1"
	// trampar com o core do networkingv1, netpols, specificamente, pra poder criar NetworkPolicies
	networkingv1 "k8s.io/api/networking/v1"
	// metav1, pra poder criar e trabalhar com ObjectMeta, LabelSelectors, etc.
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	// runtime, pra poder criar e trabalhar com Schemes, OwnerReferences, etc.
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	// trampar com controller-runtime, pra poder criar e trabalhar com Controllers, Reconciles, Managers, etc. Usa o client-go por baixo dos panos.
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	// interface de log do controller-runtime, pra poder logar mensagens de debug/info/warn/error
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	
	// karpenter modules
	karpenterv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	IngressSourceLabelKey   string
	IngressSourceLabelValue string
	ConfigMapName           string
	ConfigMapNamespace      string
	EnableNodePool          bool
}

type TenantReconcilerConfig struct {
	InstaceTypes []string `yaml:"instanceTypes"`
	MinSize      int      `yaml:"minSize"`
	MaxSize      int      `yaml:"maxSize"`
}

// +kubebuilder:rbac:groups=multitenancy.tenantforge.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=multitenancy.tenantforge.io,resources=tenants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=multitenancy.tenantforge.io,resources=tenants/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete

// Reconcile a funcao principal do controller, que é chamada sempre que um Tenant é criado/atualizado/deletado. Ela é responsável por criar/atualizar/deletar os recursos associados ao Tenant (Namespace, NetworkPolicy, ResourceQuota) de acordo com o spec do Tenant.
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("Reconciling Tenant", "Name", req.NamespacedName)

	var cm corev1.ConfigMap
	if err := r.Get(ctx, types.NamespacedName{Name: r.ConfigMapName, Namespace: r.ConfigMapNamespace}, &cm); err != nil {
		log.Error(err, "Failed to get ConfigMap", "ConfigMap", r.ConfigMapName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var configs map[string]string
	if err := yaml.Unmarshal([]byte(cm.Data["config.yaml"]), &configs); err != nil {
		log.Error(err, "Failed to unmarshal ConfigMap data", "ConfigMap", r.ConfigMapName)
		return ctrl.Result{}, err
	}
	log.Info("Configs loaded from ConfigMap", "Configs", configs)

	if r.EnableNodePool && tenant.Spec.NodePool {
		// Create or update NodePool for the tenant
		nodePool := 
		log.Info("Creating or updating NodePool for Tenant", "Tenant", tenant.Name)
		if err := controllerutil.CreateOrUpdate()
	}

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
			"tenantforge.io/tenant": tenant.Name,
			"istio-injection":       "enabled",
		}
		return controllerutil.SetControllerReference(&tenant, ns, r.Scheme)
	})

	if err != nil {
		log.Error(err, "Failed to create or update Namespace", "Namespace", tenantNamespace)
		return ctrl.Result{}, err
	}
	log.Info("Successfully reconciled Namespace", "Namespace", tenantNamespace, "Result", result)

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
			// So defino as specs pós get, pra não perder por sobrescrever oq ja tinha no netpol anteriormente
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
		log.Info("Successfully reconciled NetworkPolicy", "NetworkPolicy", netpol.Name, "Result", result)
	} else {
		log.Info("Tenant does not require network isolation, skipping NetworkPolicy creation", "Tenant", tenant.Name)
	}

	// Configure ResourceQuota based on TenantTier
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenant.Name + "-resource-quota",
			Namespace: tenantNamespace,
		},
	}

	var limit corev1.ResourceList
	switch tenant.Spec.TenantTier {
	case multitenancyv1alpha1.TenantTierLarge:
		limit = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("4"),
			corev1.ResourceMemory: resource.MustParse("8Gi"),
		}
	case multitenancyv1alpha1.TenantTierMedium:
		limit = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		}
	case multitenancyv1alpha1.TenantTierSmall:
		limit = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		}
	}
	result, err = controllerutil.CreateOrUpdate(ctx, r.Client, quota, func() error {
		quota.Spec.Hard = limit
		return controllerutil.SetControllerReference(&tenant, quota, r.Scheme)
	})
	if err != nil {
		log.Error(err, "Failed to create or update ResourceQuota for Tenant", "ResourceQuota", quota.Name)
		return ctrl.Result{}, err
	}
	log.Info("Successfully reconciled ResourceQuota", "ResourceQuota", quota.Name, "Result", result)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&multitenancyv1alpha1.Tenant{}).
		Owns(&corev1.Namespace{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&corev1.ResourceQuota{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
				if a.GetName() != r.ConfigMapName || a.GetNamespace() != r.ConfigMapNamespace {
					return []reconcile.Request{}
				}
				logf.Log.Info("ConfigMap '" + r.ConfigMapName + "' changed, reconciling all Tenants")
				// When the ConfigMap changes, we want to reconcile all Tenants
				var tenantList multitenancyv1alpha1.TenantList
				if err := r.List(ctx, &tenantList); err != nil {
					logf.Log.Error(err, "Failed to list Tenants for ConfigMap change")
					return []reconcile.Request{}
				}
				var requests []reconcile.Request
				for _, tenant := range tenantList.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: tenant.Name,
						},
					})
				}
				return requests
			}),
		).
		Named("tenant").
		Complete(r)
}
