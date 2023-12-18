## Kube Exposer

Kube Exposer is a Kubernetes custom controller written in Go using the client-go library.

It watches for any new Deployment created in any namespace and automatically generates a Service and Ingress to expose the Deployment pods externally. 
When the Deployment is deleted, it cleans up the associated Service and Ingress as well.

## Features
- Watches Deployments across all namespaces in the Kubernetes cluster
- Automatically creates a Service for each new Deployment
- Creates an Ingress resource to expose the Deployment
- Deletes Service & Ingress when parent Deployment is removed
- Built using Go and client-go for native Kubernetes integration

## Customizing

The Go source allows configuring aspects like:
- Namespace filtering
- Ingress host and path formatting
- Service port configuration
