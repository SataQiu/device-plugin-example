# A simple example for Kubernetes Device Plugin

Blog: https://blog.csdn.net/shida_csdn/article/details/137683216

## Build Image

```bash
make build
```

## Deploy to Kubernetes cluster

```bash
kubectl apply -f ./manifests/daemonset.yaml
```
