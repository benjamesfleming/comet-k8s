**Create a regional key-pair:**

```bash
$ export AWS_REGION=us-east-1
$ aws ec2 create-key-pair \
    --region $AWS_REGION \
    --key-name $AWS_REGION-k3s-key \
    --query 'KeyMaterial' \
    --output text > ~/.aws/keys/$AWS_REGION-k3s-key.pem
```

**Deploy cluster:**

```bash
$ npx cdk deploy
```