# Comet Server Operator Helm Chart

Deploy a Comet Server operator for automatically managing Comet Server deployments.

**Requirements:**

Please ensure that you have `kubectl` and `helm` installed before continuing.

**Usage:**

Add the repo and deploy the chart. See [./values.yaml](./values.yaml) for extra configuration options.

```bash
helm repo add comet-k8s https://benjamesfleming.github.io/comet-k8s

helm install comet-server-operator comet-k8s/comet-server-operator

cat <<EOF > kubectl -f -
apiVersion: cometd.cometbackup.com/v1alpha1
kind: CometLicenseIssuer
metadata:
  name: cometlicenseissuer-default
spec:
  auth:
    email: user@example.com # https://account.cometbackup.com User email 
    token: abc123           # https://account.cometbackup.com API token with the 'license::create' permission
---
apiVersion: cometd.cometbackup.com/v1alpha1
kind: CometServer
metadata:
  name: cometserver-1
  namespace: default
spec:
  version: 23.6.3
  license:
    issuer: cometlicenseissuer-default
    # TODO: License feautre flag management is not currently implemented
    # features:
    #   LIFT_STORAGE_ROLE: 0
  ingress:
    host: example.com
---
apiVersion: cometd.cometbackup.com/v1alpha1
kind: CometServer
metadata:
  name: cometserver-2
  namespace: default
spec:
  version: 23.5.2
  license:
    issuer: cometlicenseissuer-default
    # TODO: License feautre flag management is not currently implemented
    # features:
    #   LIFT_STORAGE_ROLE: 0
  ingress:
    host: example.com
EOF
```