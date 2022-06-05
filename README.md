**Create a regional key-pair:**

```bash
$ export AWS_REGION=us-east-1
$ aws ec2 create-key-pair \
    --region $AWS_REGION \
    --key-name $AWS_REGION-k3s-key \
    --query 'KeyMaterial' \
    --output text > ~/.aws/keys/$AWS_REGION-k3s-key.pem
```

**Deploy & destroy cluster:**

```bash
$ ./deploy.sh
$ ./destroy.sh
```

```bash
mkdir -p /var/lib/rancher/k3s/server/manifests

curl https://raw.githubusercontent.com/longhorn/longhorn/v1.2.4/deploy/longhorn.yaml > /var/lib/rancher/k3s/server/manifests/longhorn.yaml
```

**Deploy AWS load balancers:**

Append this to the end of the user-script to setup the [aws-load-balancer-controller](https://kubernetes-sigs.github.io/aws-load-balancer-controller/v2.4/). This requires the AWS VPC CNI, which I was unable to get working with k3s :(

```bash
# create or replace registry secret
# https://stackoverflow.com/a/55658863

ECR_REGION=us-west-2
ECR_TOKEN=$(aws ecr --region=$ECR_REGION get-authorization-token --output text --query authorizationData[].authorizationToken | base64 -d | cut -d: -f2)

kubectl delete secret --ignore-not-found -n kube-system aws-ecr-secret
kubectl create secret docker-registry aws-ecr-secret \
    -n kube-system \
    --docker-server=https://602401143452.dkr.ecr.$ECR_REGION.amazonaws.com \
    --docker-username=AWS \
    --docker-password="\${ECR_TOKEN}" \
    --docker-email="email@email.com"

# install the aws load balancer controller
# https://kubernetes-sigs.github.io/aws-load-balancer-controller/v2.4/

cat <<EOF | kubectl apply -f -
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: aws-load-balancer-controller
  namespace: kube-system
spec:
  repo: https://aws.github.io/eks-charts
  chart: eks/aws-load-balancer-controller
  targetNamespace: kube-system
  valuesContent: |
    clusterName: default
    
    serviceAccount:
      create: false
      name: aws-load-balancer-controller
      annotations:
        eks.amazonaws.com/role-arn: "${k3sAutoScalingGroup.role.roleArn}"
    
    imagePullSecrets:
    - name: aws-ecr-secret
EOF
```