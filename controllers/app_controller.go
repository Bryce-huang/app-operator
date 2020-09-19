/*


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
	k8sv1 "app-operator/api/v1"
	"context"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k8s.bryce-huang.club,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.bryce-huang.club,resources=apps/status,verbs=get;update;patch

func (r *AppReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("app", req.NamespacedName)

	//

	app := &k8sv1.App{}

	err := r.Client.Get(ctx, req.NamespacedName, app)

	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("App resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get App")
		return ctrl.Result{}, err
	}

	// find deployment
	fd := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, fd)
	if err != nil && errors.IsNotFound(err) {
		fd = r.deployment(app)
		log.Info("Creating a new Deployment", "Deployment.Namespace", fd.Namespace, "Deployment.Name", fd.Name)
		err = r.Create(ctx, fd)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", fd.Namespace, "Deployment.Name", fd.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// check images
	image := app.Spec.Image
	fImage := fd.Spec.Template.Spec.Containers[0].Image
	if fImage != image {
		fd.Spec.Template.Spec.Containers[0].Image = image
		err = r.Update(ctx, fd)
		if err != nil {
			log.Error(err, "Failed to update Deployment image", "Deployment.Namespace", fd.Namespace, "Deployment.Name", fd.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}
	// check replicas

	if *fd.Spec.Replicas != *app.Spec.Replicas {
		fd.Spec.Replicas = app.Spec.Replicas
		err = r.Update(ctx, fd)
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", fd.Namespace, "Deployment.Name", fd.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// find service
	fs := &corev1.Service{}

	err = r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, fs)
	if err != nil && errors.IsNotFound(err) {
		fs = r.service(app)
		log.Info("Creating a new Service", "Service.Namespace", fs.Namespace, "Service.Name", fs.Name)
		err = r.Create(ctx, fs)
		if err != nil {
			log.Error(err, "Failed to create new Service", "Service.Namespace", fs.Namespace, "Service.Name", fs.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// check port

	if app.Spec.Port != fs.Spec.Ports[0].Port {
		fs.Spec.Ports[0].Port = app.Spec.Port
		fs.Spec.Ports[0].TargetPort = intstr.IntOrString{Type: 0, IntVal: app.Spec.Port}
		err = r.Update(ctx, fs)
		if err != nil {
			log.Error(err, "Failed to update Service", "Service.Namespace", fs.Namespace, "Service.Name", fs.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// find ingress
	fi := &extv1.Ingress{}

	err = r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, fi)
	if err != nil && errors.IsNotFound(err) {
		fi = r.ingress(app)
		log.Info("Creating a new Ingress", "Ingress.Namespace", fi.Namespace, "Ingress.Name", fi.Name)
		err = r.Create(ctx, fi)
		if err != nil {
			log.Error(err, "Failed to create new Ingress", "Ingress.Namespace", fi.Namespace, "Ingress.Name", fi.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Ingress")
		return ctrl.Result{}, err
	}

	// check ingress port

	if app.Spec.Port != fi.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntVal {
		fi.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort = intstr.IntOrString{Type: 0, IntVal: app.Spec.Port}
		err = r.Update(ctx, fi)
		if err != nil {
			log.Error(err, "Failed to update Ingress", "Ingress.Namespace", fi.Namespace, "Ingress.Name", fi.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// update App status

	app.Status.IngressStatus = fi.Status
	app.Status.DeployStatus = fd.Status
	app.Status.SvcStatus = fs.Status

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(app.Namespace),
		client.MatchingLabels(map[string]string{"app": app.Name}),
	}

	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Memcached.Namespace", app.Namespace, "Memcached.Name", app.Name)
		return ctrl.Result{}, err
	}
	if app.Status.Pods == nil {
		appPodStatus(podList, app)
	} else if checkPodName(getPodNames(podList.Items), app.Status.Pods) {
		app.Status.Pods = nil
		appPodStatus(podList, app)
	}
	err = r.Status().Update(ctx, app)
	if err != nil {
		log.Error(err, "Failed to update App status")
		return ctrl.Result{}, err
	}

	// 结束之前
	return ctrl.Result{}, nil
}

func appPodStatus(podList *corev1.PodList, app *k8sv1.App) {
	for _, pod := range podList.Items {
		po := new(k8sv1.Pod)
		ps := pod.Status
		po.Name = pod.Name
		po.Ip = ps.PodIP
		node := new(k8sv1.Node)
		node.Ip = ps.HostIP
		node.Name = ps.NominatedNodeName
		po.Node = *node
		app.Status.Pods = append(app.Status.Pods, *po)
	}
}

func checkPodName(names []string, pods []k8sv1.Pod) bool {

	for i := range names {
		has := false
	in:
		for i2 := range pods {
			if i2 == i {
				has = true
				break in
			}
		}
		if has {
			return false
		}
	}
	return true
}
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

func labelsForApp(name string, typz string) map[string]string {
	return map[string]string{"app": name, typz: name}
}

// 构造一个deployment
func (r *AppReconciler) deployment(app *k8sv1.App) *appsv1.Deployment {
	ls := labelsForApp(app.Name, "deployment")
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: app.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: app.Spec.Image,
							Name:  app.Name,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: app.Spec.Port,
									Name:          app.Name,
								},
							},
						},
					},
				},
			},
		},
	}
	deploy.SetLabels(ls)
	_ = ctrl.SetControllerReference(app, deploy, r.Scheme)
	return deploy
}

// 构造一个Service
func (r *AppReconciler) service(app *k8sv1.App) *corev1.Service {
	ls := labelsForApp(app.Name, "service")

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       app.Spec.Port,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{Type: 0, IntVal: app.Spec.Port},
					Name:       app.Name,
				},
			},
			Selector:        map[string]string{"app": app.Name},
			SessionAffinity: corev1.ServiceAffinityNone,
			Type:            corev1.ServiceTypeClusterIP,
		},
	}
	svc.SetLabels(ls)
	_ = ctrl.SetControllerReference(app, svc, r.Scheme)
	return svc

}

// 构造一个Service
func (r *AppReconciler) ingress(app *k8sv1.App) *extv1.Ingress {
	ingress := &extv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: app.Namespace,
			Name:      app.Name,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: extv1.IngressSpec{
			Rules: []extv1.IngressRule{
				{
					Host: app.Spec.Host,
					IngressRuleValue: extv1.IngressRuleValue{
						HTTP: &extv1.HTTPIngressRuleValue{
							Paths: []extv1.HTTPIngressPath{
								{
									Path: app.Spec.Context,
									Backend: extv1.IngressBackend{
										ServicePort: intstr.IntOrString{Type: 0, IntVal: app.Spec.Port},
										ServiceName: app.Name,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	ls := labelsForApp(app.Name, "ingress")
	ingress.SetLabels(ls)
	_ = ctrl.SetControllerReference(app, ingress, r.Scheme)
	return ingress
}
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1.App{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&extv1.Ingress{}).
		Complete(r)
}
