package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	appinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	applisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiErr "k8s.io/apimachinery/pkg/api/errors"
)

type Controller struct {
	clientset           kubernetes.Interface
	deploymentLister    applisters.DeploymentLister
	deploymentCacheSync cache.InformerSynced
	queue               workqueue.RateLimitingInterface
}

func NewController(clientset kubernetes.Interface, deploymentInformer appinformers.DeploymentInformer) *Controller {
	c := &Controller{
		clientset:           clientset,
		deploymentLister:    deploymentInformer.Lister(),
		deploymentCacheSync: deploymentInformer.Informer().HasSynced,
		queue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "kube-expose"),
	}

	deploymentInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.HandleAdd,
			DeleteFunc: c.HandleDelete,
		},
	)

	return c
}

func (c *Controller) Run(stopCh <-chan struct{}) {
	fmt.Println("=======Starting controller========")
	if !cache.WaitForCacheSync(stopCh, c.deploymentCacheSync) {
		fmt.Println("Waiting for the cache to be synced")
	}

	go wait.Until(c.worker, 1*time.Second, stopCh)
	<-stopCh
	c.queue.ShutDown()
	fmt.Println("Shutting down controller")
	return

}

func (c *Controller) worker() {
	for c.processItem() {

	}
}

func (c *Controller) processItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(item)

	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		fmt.Printf("Error getting key from item %s \n", err.Error())
		return false
	}

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Printf("Error splitting key %s\n", err.Error())
		return false
	}

	// check if the object has been deleted from the API Server by querying API Server.
	ctx := context.Background()
	_, err = c.clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	// if the object is not found, it has been deleted from the API Server.
	if apiErr.IsNotFound(err) {
		fmt.Printf("Deployment %s has been deleted\n", name)

		fmt.Printf("Deleting service %s\n", name)

		// delete the service created
		err := c.clientset.CoreV1().Services(ns).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("Error deleting service %s\n", err.Error())
			return false
		}

		fmt.Printf("Deleting ingress %s\n", name)

		// delete ingress
		err = c.clientset.NetworkingV1().Ingresses(ns).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("Error deleting ingress %s\n", err.Error())
			return false
		}
		return true
	}
	
	err = c.syncDeployment(ns, name)
	if err != nil {
		fmt.Printf("Error syncing deployment %s\n", err.Error())
		return false
	}
	return true
}

func (c *Controller) syncDeployment(ns, name string) error {

	dep, err := c.deploymentLister.Deployments(ns).Get(name)
	if err != nil {
		fmt.Printf("Error getting deployment %s\n", err.Error())
	}


	//create svc
	ctx := context.Background()
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: dep.Name,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Type: "LoadBalancer",
			Selector: depLabels(*dep),
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
		},
	}
	s, err := c.clientset.CoreV1().Services(ns).Create(ctx, &svc, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	//create ingress
	return createIngress(ctx, c.clientset, s)
}

func createIngress(ctx context.Context, clientset kubernetes.Interface, svc *corev1.Service) error {
	pathType := "Prefix"
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: svc.Name,
			Namespace: svc.Namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target":"/",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: fmt.Sprintf("/%s", svc.Name),
									PathType: (*networkingv1.PathType)(&pathType),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: svc.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
												

		},
	}
	_, err := clientset.NetworkingV1().Ingresses(svc.Namespace).Create(ctx, &ingress, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
} 

func depLabels(dep appsv1.Deployment) map[string]string {
	return dep.Spec.Template.Labels
}


func (c *Controller) HandleAdd(obj interface{}) {
	fmt.Println("Add was called")
	c.queue.Add(obj)

}

func (c *Controller) HandleDelete(obj interface{}) {
	fmt.Println("Delete was called")
	c.queue.Add(obj)
}
