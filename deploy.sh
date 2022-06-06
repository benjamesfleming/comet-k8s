#!/bin/bash
cd ./infra && npx cdk deploy

IP=$(aws ec2 describe-instances \
        --region $AWS_REGION \
        --filters Name=instance-state-name,Values=running \
        --query 'sort_by(Reservations[].Instances[], &LaunchTime)[*].[PublicIpAddress]' \
        --output text | head -n1)

echo "ssh -i ~/.ssh/$AWS_REGION-k3s-key.pem ec2-user@$IP"
echo "pulling kubeconfig"

until scp -i ~/.ssh/$AWS_REGION-k3s-key.pem ec2-user@$IP:/etc/rancher/k3s/k3s1.yaml ./kubeconfig.yaml; do
    sleep 5
done

sed -i s/127.0.0.1/$IP/g ./kubeconfig.yaml

export KUBECONFIG=$(pwd)/kubeconfig.yaml

kubectl get nodes --insecure-skip-tls-verify