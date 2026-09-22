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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	multitenancyv1alpha1 "github.com/sant125/tenantforge/api/v1alpha1"
	// karpenter modules
	karpenterapisv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

// NodePoolProvisioner abstrai como o "node pool" de um tenant é provisionado.
// Cada mecanismo resolve isso de um jeito diferente: o Karpenter cria um CRD
// (NodePool) dentro do próprio cluster e outro controller fica de olho nele -
// é o que a implementação abaixo faz. O GKE, por exemplo, não tem esse CRD:
// pra mexer no node pool dele tem que chamar a API do GCP direto (SDK, fora
// do client-go/controller-runtime). O TenantReconciler só enxerga essa
// interface, nunca o mecanismo por trás - trocar de implementação não muda
// uma linha do Reconcile.
type NodePoolProvisioner interface {
	// Reconcile garante que o node pool do tenant existe e está com a config
	// atual. Deve ser idempotente, igual o resto do reconcile do Tenant.
	Reconcile(ctx context.Context, tenant *multitenancyv1alpha1.Tenant, cfg NodePoolConfig) error
}

// KarpenterNodePoolProvisioner implementa NodePoolProvisioner criando/atualizando
// um karpenterapisv1.NodePool (CRD do Karpenter) por tenant.
//
// Fica agnóstico de cloud provider porque quem realmente sabe criar a
// instância/VM é o NodeClass referenciado em Spec.Template.Spec.NodeClassRef -
// e o Group/Kind/Name dele vêm inteiros da NodePoolConfig (ConfigMap), nunca
// hardcoded aqui. Em AWS isso aponta pra um EC2NodeClass (karpenter.k8s.aws),
// em Azure pra um AKSNodeClass (karpenter.azure.com), testando local no kind
// aponta pro KWOKNodeClass (karpenter.kwok.sh) do provider de teste oficial
// do próprio Karpenter. Esse provisioner não muda em nenhum desses casos.
type KarpenterNodePoolProvisioner struct {
	client.Client
	Scheme *runtime.Scheme
}

func (p *KarpenterNodePoolProvisioner) Reconcile(ctx context.Context, tenant *multitenancyv1alpha1.Tenant, cfg NodePoolConfig) error {
	log := logf.FromContext(ctx)

	nodePool := &karpenterapisv1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: tenant.Name,
		},
	}
	result, err := controllerutil.CreateOrUpdate(ctx, p.Client, nodePool, func() error {
		nodePool.Labels = map[string]string{"tenantforge.io/tenant": tenant.Name}
		nodePool.Spec = karpenterapisv1.NodePoolSpec{
			Template: karpenterapisv1.NodeClaimTemplate{
				Spec: karpenterapisv1.NodeClaimTemplateSpec{
					NodeClassRef: &karpenterapisv1.NodeClassReference{
						Group: cfg.NodeClassGroup,
						Kind:  cfg.NodeClassKind,
						Name:  cfg.NodeClassName,
					},
					Requirements: []karpenterapisv1.NodeSelectorRequirementWithMinValues{
						{
							Key:      corev1.LabelInstanceTypeStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   cfg.InstanceTypes,
						},
					},
					Taints: []corev1.Taint{
						{
							Key:    "tenantforge.io/tenant",
							Value:  nodePool.Name,
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			// Sem Weight/Replicas: NodePool fica em modo dinâmico (Karpenter
			// escala por demanda de pods, que é o objetivo aqui). Setar os
			// dois juntos, mesmo zerados, é rejeitado pela API - tem uma
			// regra CEL no CRD que barra "weight" quando "replicas" existe.
			Limits: karpenterapisv1.Limits{
				corev1.ResourceName("nodes"): resource.MustParse(strconv.Itoa(cfg.MaxNodes)),
			},
		}
		return controllerutil.SetControllerReference(tenant, nodePool, p.Scheme)
	})
	if err != nil {
		log.Error(err, "Failed to create or update NodePool", "NodePool", nodePool.Name)
		return err
	}
	log.Info("Successfully reconciled NodePool", "NodePool", nodePool.Name, "Result", result)
	return nil
}
