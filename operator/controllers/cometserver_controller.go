/*
Copyright 2023.

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
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cometdv1alpha1 "github.com/cometbackup/comet-server-operator/api/v1alpha1"
	"github.com/go-logr/logr"
)

const (
	cometServerFinalizer    = "cometd.cometbackup.com/finalizer"
	cometServerLabel        = "cometd.cometbackup.com/pod-name"
	cometServerSerialNumber = "cometd.cometbackup.com/serial-number"
)

// CometServerReconciler reconciles a CometServer object
type CometServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cometd.cometbackup.com,resources=cometservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cometd.cometbackup.com,resources=cometservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cometd.cometbackup.com,resources=cometservers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CometServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *CometServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx).WithValues("cometserver", req.NamespacedName)
	reqLogger.Info("Reconciling CometServer")

	// Fetch the CometServer instance
	cs := &cometdv1alpha1.CometServer{}
	err := r.Get(ctx, req.NamespacedName, cs)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("CometServer resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get CometServer.")
		return ctrl.Result{}, err
	}

	// Check if the CometServer instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isMarkedToBeDeleted := cs.GetDeletionTimestamp() != nil
	if isMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(cs, cometServerFinalizer) {
			// Run finalization logic for memcachedFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeCometServer(reqLogger, cs); err != nil {
				return ctrl.Result{}, err
			}

			// Remove memcachedFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(cs, cometServerFinalizer)
			err := r.Update(ctx, cs)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// --

	err = r.reconcileCometServer(context.TODO(), reqLogger, cs)
	if err != nil {
		reqLogger.Error(err, "Failed to create/update cometserver resources.")
		return ctrl.Result{}, nil
	}
	_ = r.Client.Status().Update(context.TODO(), cs)
	return ctrl.Result{}, nil
}

func (r *CometServerReconciler) reconcileCometServer(ctx context.Context, reqLogger logr.Logger, cs *cometdv1alpha1.CometServer) error {
	// License
	if _, ok := cs.Annotations[cometServerSerialNumber]; !ok {
		// Serial Number not defined as an annotation - this must be a first start up.
		// Attempt to generate a new serial number using the defined CometServerLicenseIssuer -
		reqLogger.Info("CometServer serial number not defined... attempting to generate a new one.")
		issuer := &cometdv1alpha1.CometLicenseIssuer{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: cs.Spec.License.Issuer, Namespace: cs.Namespace}, issuer)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to get cometlicenseissuer/%s - It must be defined before CometServer resource creation.", cs.Spec.License.Issuer))
			return err
		}
		serial, err := newSerialNumber(issuer, &cs.Spec.License.Features)
		if err != nil {
			reqLogger.Error(err, "Failed to generate new serial number.")
			return err
		}
		// Add the serial number as an annotation
		if cs.Annotations == nil {
			cs.Annotations = make(map[string]string)
		}
		cs.Annotations[cometServerSerialNumber] = serial
		if err := r.Client.Update(ctx, cs); err != nil {
			reqLogger.Error(err, "Failed to add serial number label.")
			return err
		}
	}

	// Service
	svcExpected := getCometServerService(cs)
	svcActual := &corev1.Service{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-service", cs.Name), Namespace: cs.Namespace}, svcActual)
	if err != nil {
		// Failed to get service - maybe we need to create it?
		if errors.IsNotFound(err) {
			controllerutil.SetControllerReference(cs, svcExpected, r.Scheme)
			err = r.Client.Create(ctx, svcExpected)
			if err != nil {
				return err
			}
		} else {
			return err
		}

	} else if !reflect.DeepEqual(svcExpected.Spec, svcActual.Spec) {
		svcExpected.ObjectMeta = svcActual.ObjectMeta
		controllerutil.SetControllerReference(cs, svcExpected, r.Scheme)
		err = r.Client.Update(ctx, svcExpected)
		if err != nil {
			return err
		}
		reqLogger.Info("Successfully updated Service")
	}

	// Ingress
	ingressExpected := getCometServerIngress(cs)
	ingressActual := &networkingv1.Ingress{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-ingress", cs.Name), Namespace: cs.Namespace}, ingressActual)
	if err != nil {
		// Failed to get ingress - maybe we need to create it?
		if errors.IsNotFound(err) {
			controllerutil.SetControllerReference(cs, ingressExpected, r.Scheme)
			err = r.Client.Create(ctx, ingressExpected)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else if !reflect.DeepEqual(ingressExpected.Spec, ingressActual.Spec) {
		ingressExpected.ObjectMeta = ingressActual.ObjectMeta
		controllerutil.SetControllerReference(cs, ingressExpected, r.Scheme)
		err = r.Client.Update(ctx, ingressExpected)
		if err != nil {
			return err
		}
		reqLogger.Info("Successfully updated Ingress")
	}

	// PersistentVolumeClaim
	pvcExpected := getCometServerPVC(cs)
	pvcActual := &corev1.PersistentVolumeClaim{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-pvc", cs.Name), Namespace: cs.Namespace}, pvcActual)
	if err != nil {
		// Failed to get pvc - maybe we need to create it?
		if errors.IsNotFound(err) {
			controllerutil.SetControllerReference(cs, pvcExpected, r.Scheme)
			err = r.Client.Create(ctx, pvcExpected)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		// PVCs are immutable after creation... nothing to do here.
	}

	// Deployment
	deplExpected := getCometServerDeployment(cs)
	deplActual := &appsv1.Deployment{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: cs.Name, Namespace: cs.Namespace}, deplActual)
	if err != nil {
		// Failed to get deployment - maybe we need to create it?
		if errors.IsNotFound(err) {
			controllerutil.SetControllerReference(cs, deplExpected, r.Scheme)
			err = r.Client.Create(ctx, deplExpected)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else if !reflect.DeepEqual(deplExpected.Spec, deplActual.Spec) {
		deplExpected.ObjectMeta = deplActual.ObjectMeta
		controllerutil.SetControllerReference(cs, deplExpected, r.Scheme)
		err = r.Client.Update(ctx, deplExpected)
		if err != nil {
			return err
		}
		reqLogger.Info("Successfully updated Deployment")
	}

	return nil
}

func (r *CometServerReconciler) finalizeCometServer(reqLogger logr.Logger, cs *cometdv1alpha1.CometServer) error {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted. Examples
	// of finalizers include performing backups and deleting
	// resources that are not owned by this CR, like a PVC.
	reqLogger.Info("Successfully finalized CometServer.")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CometServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cometdv1alpha1.CometServer{}).
		Complete(r)
}

// --

func getCometServerService(cs *cometdv1alpha1.CometServer) *corev1.Service {
	labels := map[string]string{"app": cs.Name}
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-service", cs.Name),
			Namespace: cs.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "web",
					Port:     8060,
					Protocol: corev1.ProtocolTCP,
				},
			},
			Selector:  labels,
			ClusterIP: "None",
		},
	}
}

func getCometServerIngress(cs *cometdv1alpha1.CometServer) *networkingv1.Ingress {
	labels := map[string]string{"app": cs.Name}
	ingressClassName := "traefik"
	return &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ingress", cs.Name),
			Namespace: cs.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"cert-manager.io/cluster-issuer": "letsencrypt-prod",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{
						fmt.Sprintf("*.%s.%s", cs.Name, cs.Spec.Ingress.Host),
						fmt.Sprintf("%s.%s", cs.Name, cs.Spec.Ingress.Host),
					},
					SecretName: "letsencrypt-prod",
				},
			},
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: fmt.Sprintf("%s-service", cs.Name),
					Port: networkingv1.ServiceBackendPort{
						Name: "web",
					},
				},
			},
		},
	}
}

func getCometServerPVC(cs *cometdv1alpha1.CometServer) *corev1.PersistentVolumeClaim {
	labels := map[string]string{"app": cs.Name}
	storageClassName := "hostpath"
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-pvc", cs.Name),
			Namespace: cs.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &storageClassName,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("8Gi"),
				},
			},
		},
	}
}

func getCometServerDeployment(cs *cometdv1alpha1.CometServer) *appsv1.Deployment {
	labels := map[string]string{"app": cs.Name}
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels,
			Annotations: cs.Annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "cometd",
					Image:           "ghcr.io/cometbackup/comet-server:" + cs.Spec.Version,
					ImagePullPolicy: "Always",
					Ports: []corev1.ContainerPort{
						{
							Name:          "web",
							ContainerPort: 8060,
						},
					},
					Env: []corev1.EnvVar{
						{
							Name: "COMET_LICENSE_SERIAL",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									// Pull the serial number from the annotations -
									// This should always be set before the deployment is created.
									FieldPath: fmt.Sprintf("metadata.annotations['%s']", cometServerSerialNumber),
								},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "cometd-data",
							MountPath: "/var/lib/cometd",
							SubPath:   "data",
						},
						{
							Name:      "cometd-data",
							MountPath: "/var/log/cometd",
							SubPath:   "logs",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "cometd-data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: fmt.Sprintf("%s-pvc", cs.Name),
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cs.Name,
			Namespace: cs.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: podTemplateSpec,
		},
	}
}

// --

type licenseCreateResponse struct {
	Data struct {
		SerialNumber string `json:"serial_number"`
	} `json:"data"`
}

func newSerialNumber(issuer *cometdv1alpha1.CometLicenseIssuer, features *cometdv1alpha1.CometLicenseFeatures) (string, error) {
	client := &http.Client{}
	data := url.Values{
		"auth_type": []string{"token"},
		"email":     []string{issuer.Spec.Auth.Email},
		"token":     []string{issuer.Spec.Auth.Token},
	}

	resp, err := client.PostForm("https://account.cometbackup.com/api/v1/license/create_license", data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		err = stderrors.New(fmt.Sprintf("Expected HTTP-200 got HTTP-%d: %s", resp.StatusCode, string(body)))
		return "", err
	}

	var result licenseCreateResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// Done! Serial number generated

	return result.Data.SerialNumber, nil
}
