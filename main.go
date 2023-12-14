package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/nishanth-gowda/kube-expose/controllers"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", home+"/.kube/config", "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalf("error %s building config from flags", err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("error %s building clientset from config", err.Error())
	}

	log.Println("clientset created")

	stopCh := make(chan struct{})
	infromer := informers.NewSharedInformerFactory(clientset, 10*time.Minute)

	controller := controllers.NewController(clientset, infromer.Apps().V1().Deployments())
	infromer.Start(stopCh)
	controller.Run(stopCh)
	fmt.Println(infromer)


}