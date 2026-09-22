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

package v1

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	// karpenter modules
	karpenterapisv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

// nolint:unused
// log is for logging in this package.
var podlog = logf.Log.WithName("pod-resource")

// SetupPodWebhookWithManager registers the webhook for Pod in the manager.
func SetupPodWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &corev1.Pod{}).
		WithDefaulter(&PodDefaulter{Client: mgr.GetClient()}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate--v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod-v1.kb.io,admissionReviewVersions=v1

// PodDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Pod when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type PodDefaulter struct {
	client.Client
}

// Default implements admission.Defaulter so a webhook will be registered for the Kind Pod.
func (d *PodDefaulter) Default(ctx context.Context, obj *corev1.Pod) error {
	podlog.Info("Defaulting for Pod", "name", obj.GetName())

	// TODO(user): fill in your defaulting logic.
	var namespace corev1.Namespace
	if err := d.Get(ctx, client.ObjectKey{Name: obj.Namespace}, &namespace); err != nil {
		return err
	}

	// Label do namespace é só um filtro rápido: "esse pod é de algum tenant".
	// Não quer dizer que esse tenant tem NodePool - tenant.Spec.NodePool é
	// opcional e default false, e o label do namespace sai igual pros dois
	// casos (ver tenant_controller.go). Sem confirmar que o NodePool existe de
	// verdade, injetaríamos NodeSelector pra um NodePool inexistente em todo
	// pod de todo tenant "normal" - deadlock, pod nunca escalona.
	tenant, ok := namespace.Labels["tenantforge.io/tenant"]
	if !ok {
		podlog.Info("Pod não pertence a nenhum tenant, não mutando")
		return nil
	}

	var nodePool karpenterapisv1.NodePool
	if err := d.Get(ctx, client.ObjectKey{Name: tenant}, &nodePool); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		podlog.Info("Tenant não tem NodePool, não mutando", "tenant", tenant)
		return nil
	}

	podlog.Info("Tenant tem NodePool, injetando NodeSelector e Toleration", "tenant", tenant)
	if obj.Spec.NodeSelector == nil {
		obj.Spec.NodeSelector = map[string]string{}
	}
	obj.Spec.NodeSelector["karpenter.sh/nodepool"] = tenant
	obj.Spec.Tolerations = append(obj.Spec.Tolerations, corev1.Toleration{
		Key:      "tenantforge.io/tenant",
		Operator: corev1.TolerationOpEqual,
		Value:    tenant,
		Effect:   corev1.TaintEffectNoSchedule,
	})
	return nil
}
