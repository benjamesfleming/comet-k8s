# Comet Server Helm Chart

Create a Comet Server deployment using [helm](https://helm.sh/).

**Requirements:**

Please ensure that you have `kubectl` and `helm` installed before continuing.

**Usage:**

1. Ensure that your cluster is working

`kubectl get nodes -o wide`

2. Configure secrets/API authentication

`kubectl create secret generic comet-api-token --namespace default --from-literal email=<email> --from-literal token=<token>`

3. Deploy the chart. See [./values.yml](./values.yml) for extra configuration options.

`helm install cometd ./`