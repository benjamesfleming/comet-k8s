# Comet Server Helm Chart

Create a Comet Server deployment using [helm](https://helm.sh/).

**Requirements:**

Please ensure that you have `kubectl` and `helm` installed before continuing.

**Usage:**

Add the repo and deploy the chart. See [./values.yaml](./values.yaml) for extra configuration options.

```bash
kubectl create secret generic comet-api-token \
  --namespace default \
  --from-literal email=<email> \
  --from-literal token=<token>

helm repo add comet-k8s https://benjamesfleming.github.io/comet-k8s

helm install cometd comet-k8s/comet-server
```