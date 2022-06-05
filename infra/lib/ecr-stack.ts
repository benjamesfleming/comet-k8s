import * as cdk from 'aws-cdk-lib';
import { Construct } from 'constructs';
import {
  aws_iam as iam,
  aws_s3 as s3,
  aws_ec2 as ec2,
  aws_autoscaling as autoscaling,
  aws_elasticloadbalancingv2 as elb2,
  aws_route53 as route53,
} from 'aws-cdk-lib';

interface Props extends cdk.StackProps {
    
}

export default class EcrStack extends cdk.Stack {
    constructor(scope: Construct, id: string, props?: Props) {
        super(scope, id, props);
        
        // TODO: setup ecr stack
    }
}