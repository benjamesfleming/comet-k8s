/*
Copyright 2022.

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/external-dns/endpoint"

	cometv1 "github.com/cometbackup/hosted-comet-operator/api/v1"
)

// HostedCometReconciler reconciles a HostedComet object
type HostedCometReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=comet.cometbackup.com,resources=hostedcomets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=comet.cometbackup.com,resources=hostedcomets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=comet.cometbackup.com,resources=hostedcomets/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;watch;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HostedComet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *HostedCometReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	hostedComet := &cometv1.HostedComet{}
	if err := r.Get(ctx, req.NamespacedName, hostedComet); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("HostedComet resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get HostedComet")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling HostedComet", "HostedComet.Namespace", hostedComet.Namespace, "HostedComet.Name", hostedComet.Name)

	// Get DNSEndpoint for HostedComet
	dnsEndpoint := &endpoint.DNSEndpoint{}
	if err := r.Get(ctx, req.NamespacedName, dnsEndpoint); err != nil && errors.IsNotFound(err) {
		// Define a new DNSEndpoint
		if _, err := r.CreateDNSEndpoint(ctx, hostedComet); err != nil {
			log.Error(err, "Failed to create new DNSEndpoint", "DNSEndpoint.Namespace", dnsEndpoint.Namespace, "DNSEndpoint.Name", dnsEndpoint.Name)
			return ctrl.Result{}, err
		}
		// DNSEndpoint created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get DNSEndpoint")
		return ctrl.Result{}, err
	}

	dnsEndpoint.Spec.Endpoints = r.GetDNSEndpoints(ctx, hostedComet)
	r.Update(ctx, dnsEndpoint)

	return ctrl.Result{}, nil
}

func (r *HostedCometReconciler) CreateDNSEndpoint(ctx context.Context, h *cometv1.HostedComet) (*endpoint.DNSEndpoint, error) {
	dnsEndpoint := &endpoint.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
		},
		Spec: endpoint.DNSEndpointSpec{
			Endpoints: r.GetDNSEndpoints(ctx, h),
		},
	}

	if err := r.Create(ctx, dnsEndpoint); err != nil {
		return nil, err
	}

	ctrl.SetControllerReference(h, dnsEndpoint, r.Scheme)

	return dnsEndpoint, nil
}

func (r *HostedCometReconciler) GetDNSEndpoints(_ context.Context, h *cometv1.HostedComet) []*endpoint.Endpoint {
	endpoints := []*endpoint.Endpoint{
		endpoint.NewEndpointWithTTL(h.GetRegionFQDN(), "A", 180, r.GetNodesTargets()...),
	}

	for i := 0; i < h.Spec.Replicas; i++ {
		fqdn := h.GetPodFQDN(i)
		endpoints = append(endpoints,
			endpoint.NewEndpointWithTTL(fqdn, "CNAME", 180, h.GetRegionFQDN()),
			endpoint.NewEndpointWithTTL("*."+fqdn, "CNAME", 180, h.GetRegionFQDN()),
		)
	}

	return endpoints
}

func (r *HostedCometReconciler) GetNodesTargets() []string {
	nodeList := &corev1.NodeList{}
	targets := []string{}

	if err := r.List(context.TODO(), nodeList); err != nil {
		log.Log.Error(err, "Failed to list nodes")
		return []string{}
	}

	// loop through node list and get all the external ips
	// this excludes nodes marked 'NotReady'

loop:
	for _, node := range nodeList.Items {
		for _, con := range node.Status.Conditions {
			if con.Type == corev1.NodeReady && con.Status != corev1.ConditionTrue {
				continue loop
			}
		}

		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeExternalIP {
				targets = append(targets, address.Address)
			}
		}
	}

	return targets
}

// SetupWithManager sets up the controller with the Manager.
func (r *HostedCometReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cometv1.HostedComet{}).
		Watches(&source.Kind{Type: &corev1.Node{}}, r.EnqueueRequestMap()).
		Complete(r)
}

// --

func (r *HostedCometReconciler) EnqueueRequestMap() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		hostedComets := &cometv1.HostedCometList{}
		if err := r.List(context.TODO(), hostedComets); err != nil {
			log.Log.Error(err, "Failed to list HostedComet resources")
			return []reconcile.Request{} // no-op by default
		}

		ret := make([]reconcile.Request, len(hostedComets.Items))
		for i := 0; i < len(ret); i++ {
			ret[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: hostedComets.Items[i].Namespace,
					Name:      hostedComets.Items[i].Name,
				},
			}
		}
		return ret
	})
}
