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
  cleanup?: boolean;
  expose?: number[];
  keyName: string;
  hostedZone?: string;
}

export class K3sStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: Props) {
    super(scope, id, props);

    // setup the configuration s3 bucket
    // the nodes will check this bucket at startup and configure accordingly
    const k3sBucket = new s3.Bucket(this, 's3-mgmt-bucket', {
      removalPolicy: props?.cleanup?cdk.RemovalPolicy.DESTROY:cdk.RemovalPolicy.RETAIN,
      autoDeleteObjects: props?.cleanup??false,
    });

    // enable versioning on the bucket, 
    // this is required for the "lock" system
    (k3sBucket.node.defaultChild as s3.CfnBucket).versioningConfiguration = {
      status: 'Enabled'
    };

    // setup the vpc
    // this will only use public subnets
    const k3sVpc = new ec2.Vpc(this, 'vpc', { 
      natGateways: 1, 
      subnetConfiguration: [
        {name: 'public', cidrMask: 24, subnetType: ec2.SubnetType.PUBLIC},
      ],
    });

    // setup the security group
    const k3sSecurityGroup = new ec2.SecurityGroup(this, 'sg', { vpc: k3sVpc });

    // add default ingress rules
    //   ec2.Port.tcp(6443)    - allow external access to the k3s api
    //   ec2.Port.allTraffic() - allow ec2 instances in the same sg to access each other
    k3sSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(6443), '');
    k3sSecurityGroup.addIngressRule(k3sSecurityGroup, ec2.Port.allTraffic(), '');
    
    // setup the k3s autoscaling group
    // this handles the lanuch template configuration
    const k3sAutoScalingGroup = new autoscaling.AutoScalingGroup(this, 'asg', {
      instanceType: ec2.InstanceType.of(ec2.InstanceClass.T4G, ec2.InstanceSize.MEDIUM),
      machineImage: ec2.MachineImage.latestAmazonLinux({
        cpuType: ec2.AmazonLinuxCpuType.ARM_64,
        generation: ec2.AmazonLinuxGeneration.AMAZON_LINUX_2,
      }),
      vpc: k3sVpc,
      securityGroup: k3sSecurityGroup,
      blockDevices: [
        {deviceName: '/dev/sdb', volume: autoscaling.BlockDeviceVolume.ebs(64)},
      ],
      maxCapacity: 2,
      minCapacity: 2,
      
      keyName: `${props?.env?.region}-k3s-key`,
    });

    // setup the network load balancer
    const k3sLoadBalancer = new elb2.NetworkLoadBalancer(this, 'elbv2', { vpc: k3sVpc });

    // add the default 6443 port to the load balancer
    // this is required to access the k3s cluster from outside the vpc
    const k3sTargetGroup6443 = k3sLoadBalancer
      .addListener('elb-port-6443', { port: 6443 })
      .addTargets('elb-target-6443', { 
        port: 6443, 
        targets: [k3sAutoScalingGroup] 
      });


    // expose the user provided ports to the load balancer
    // FIXME: this defaults to TCP connections only
    for (let port of props?.expose??[]) {
      k3sSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(port), 'port-tcp-'+port);
      k3sLoadBalancer
        .addListener('listener-'+port, { port })
        .addTargets('target-'+port, { port, targets: [k3sAutoScalingGroup] });
    }

    // add the instance user data
    // this is a bash script that is run on each instances inital start up
    k3sAutoScalingGroup.addUserData(
`
#!/bin/bash -xe
exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1

export LOGFILE='/var/log/k3s.log'
export BUCKET_NAME=${k3sBucket.bucketName}
export ELB_TARGET_GROUP_ARN=${k3sTargetGroup6443.targetGroupArn}

# register ip in s3 bucket

MY_ID=$(curl http://169.254.169.254/latest/meta-data/instance-id)
MY_IP=$(curl http://169.254.169.254/latest/meta-data/local-ipv4)

echo $MY_IP | aws s3 cp - s3://$BUCKET_NAME/nodes/$MY_ID

# hacky locking system
# --
# all devices will update the <bucket>/lock file. 
# the first device to obtain a lock (the oldest version) is consisdered to have the lock

MY_LOCK_ID=$(aws s3api put-object --bucket $BUCKET_NAME --key lock --body /etc/hostname --output text --query 'VersionId')
OLDEST_LOCK_ID=$(aws s3api list-object-versions --bucket $BUCKET_NAME --prefix lock --no-paginate --output text --query 'Versions[-1].VersionId')

echo "my lock: $MY_LOCK_ID"
echo "oldest lock: $OLDEST_LOCK_ID"

if [ "$MY_LOCK_ID" != "$OLDEST_LOCK_ID" ]; then 
  echo "waiting for cluster to start up"

  # wait for the initial node to init the cluster
  # check the load balancer target group, connect to the first healthy node

  NODE_0_ID=$(aws elbv2 describe-target-health --region ${props?.env?.region} --target-group-arn $ELB_TARGET_GROUP_ARN --query 'TargetHealthDescriptions[?TargetHealth.State==\`healthy\`].Target.Id | [0]' --output text)
  while [ "$NODE_0_ID" == "None" ]; do
    sleep 5
    NODE_0_ID=$(aws elbv2 describe-target-health --region ${props?.env?.region} --target-group-arn $ELB_TARGET_GROUP_ARN --query 'TargetHealthDescriptions[?TargetHealth.State==\`healthy\`].Target.Id | [0]' --output text)
  done
  
  NODE_0_IP=$(aws s3 cp s3://$BUCKET_NAME/nodes/$NODE_0_ID -)
  TOKEN=$(aws s3 cp s3://$BUCKET_NAME/token -)

  curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=stable INSTALL_K3S_EXEC="server -t $TOKEN --server https://$NODE_0_IP:6443 --write-kubeconfig-mode 0644" sh -s -
else
  echo "assuming node0 role... creating cluster"

  # lock acquired, start cluster init process
  # this will generate the cluster token

  curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=stable INSTALL_K3S_EXEC="server --cluster-init --write-kubeconfig-mode 0644" sh -s -

  cp /etc/rancher/k3s/k3s.yaml /tmp/kubeconfig.yaml
  sed -i s/127.0.0.1/${props?.env?.region}.k3s.hostedsrv.net/ /tmp/kubeconfig.yaml

  aws s3 cp /var/lib/rancher/k3s/server/node-token s3://$BUCKET_NAME/token
  aws s3 cp /tmp/kubeconfig.yaml s3://$BUCKET_NAME/kubeconfig.yaml
fi
`);

    // set up the default instance role
    // this give the instances access to the management bucket and elb target health, which
    // is needed for initial cluster coordination
    k3sAutoScalingGroup.role.attachInlinePolicy(
      new iam.Policy(this, 'k3s-instance-policy', {
        statements: [
          new iam.PolicyStatement({
            // https://docs.aws.amazon.com/elasticloadbalancing/latest/userguide/load-balancer-authentication-access-control.html#elb-resources
            // DescribeTargetHealth requires wildcard '*' resources
            resources: ['*'],
            actions: [
              'elasticloadbalancing:DescribeTargetHealth'
            ],
          }),
          new iam.PolicyStatement({
            resources: [
              k3sBucket.bucketArn,
              k3sBucket.bucketArn+'/*'
            ],
            actions: [
              's3:Abort*',
              's3:DeleteObject*',
              's3:GetBucket*',
              's3:GetObject*',
              's3:List*',
              's3:PutObject',
              's3:PutObjectLegalHold',
              's3:PutObjectRetention',
              's3:PutObjectTagging',
              's3:PutObjectVersionTagging',
              's3:PutObjectAcl',
              's3:PutObjectVersionAcl',
            ],
          }),
        ]
      })
    );

    if (props?.hostedZone !== undefined) {

      // setup route 53
      // *.{region}.k3s.{domain} -> {loadBalancerDnsName}
      //   {region}.k3s.{domain} -> {loadBalancerDnsName}

      let zone = route53.HostedZone.fromLookup(
        this, 'hostedzone', { domainName: props.hostedZone }
      );
      
      let domainName = k3sLoadBalancer.loadBalancerDnsName;
      let recordName = `${props?.env?.region}.k3s.${zone.zoneName}.`;

      new route53.CnameRecord(this, 'k3s-cname', {zone, domainName, recordName});
      new route53.CnameRecord(this, 'k3s-wildcard-cname', {zone, domainName, recordName: '*.'+recordName});

      new cdk.CfnOutput(this, 'Endpoint', { value: `https://${recordName}:6443` });
    } else {
      new cdk.CfnOutput(this, 'Endpoint', { value: `https://${k3sLoadBalancer.loadBalancerDnsName}:6443` });
    }
    
    new cdk.CfnOutput(this, 'Kubernetes Configuration File', { value: `s3://${k3sBucket.bucketName}/kubeconfig.yaml` });
  }
}
