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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			Type: "ClusterIP",
			Selector: depLabels(*dep),
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
		},
	}
	_, err = c.clientset.CoreV1().Services(ns).Create(ctx, &svc, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil

	//create ingress
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