#!/usr/bin/env node
import { App, aws_ec2 as ec2 } from 'aws-cdk-lib';
import { K3sStack } from '../lib/k3s-cluster-stack';

const app = new App();

new K3sStack(app, 'cdk-k3s-cluster-us-east-1', {
  env: { 
    // Use the AWS Account that is implied by
    // the current CLI configuration.
    account: process.env.CDK_DEFAULT_ACCOUNT, 
    region: 'us-east-1',
  },
  cleanup: true,
  expose: [80, 443],
  // instance settings
  // increase the instanceCount to add mode nodes to the cluster
  instanceCount: 3,
  instanceType: ec2.InstanceType.of(
    ec2.InstanceClass.T4G, ec2.InstanceSize.MEDIUM
  ),
  // key pairs must be created manually
  // $ export AWS_REGION=us-east-1
  // $ aws ec2 create-key-pair --region $AWS_REGION --key-name $AWS_REGION-k3s-key --query 'KeyMaterial' --output text > $AWS_REGION-k3s-key.pem
  keyName: 'us-east-1-k3s-key',
  // pre-registed domain to use. (remove this to use the elb endpoint)
  // stack will genearte the following records
  //  CNAME *.{region}.k3s.{domain} -> {loadBalancerDnsName}
  //  CNAME   {region}.k3s.{domain} -> {loadBalancerDnsName}
  hostedZone: 'hostedsrv.net',
});