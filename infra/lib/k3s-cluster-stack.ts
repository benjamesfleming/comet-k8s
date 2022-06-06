import * as cdk from 'aws-cdk-lib';
import { Construct } from 'constructs';
import {
    aws_iam as iam,
    aws_ec2 as ec2,
    aws_autoscaling as autoscaling,
    aws_secretsmanager as secretsmanager,
} from 'aws-cdk-lib';
import { AWS_ROUTE53_POLICY } from '../policies/aws-route53-external-dns';

interface Props extends cdk.StackProps {
    cleanup?: boolean;
    expose?: number[];
    instanceCount: number;
    instanceType: ec2.InstanceType;
    keyName: string;
    hostedZone?: string;
}

export class K3sStack extends cdk.Stack {
    constructor(scope: Construct, id: string, props?: Props) {
        super(scope, id, props);

        // setup the vpc
        // this will only use public subnets
        const k3sVpc = new ec2.Vpc(this, 'vpc', { 
            natGateways: 1, 
            maxAzs: 1,
            subnetConfiguration: [
                {name: 'ingress', cidrMask: 24, subnetType: ec2.SubnetType.PUBLIC},
            ],
        });

        // setup the security group
        const k3sSecurityGroup = new ec2.SecurityGroup(this, 'sg', { vpc: k3sVpc });

        // add default ingress rules
        //   ec2.Port.tcp(22)      - allow external ssh access
        //   ec2.Port.tcp(80)      - allow external access traefik:80
        //   ec2.Port.tcp(443)     - allow external access traefik:443
        //   ec2.Port.tcp(6443)    - allow external access to the k3s api
        //   ec2.Port.allTraffic() - allow ec2 instances in the same sg to access each other
        k3sSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(22), '');
        k3sSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(80), '');
        k3sSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(443), '');
        k3sSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(6443), '');
        k3sSecurityGroup.addIngressRule(k3sSecurityGroup, ec2.Port.allTraffic(), '');
        
        // setup the k3s autoscaling group
        // this handles the lanuch template configuration
        const k3sAutoScalingGroup = new autoscaling.AutoScalingGroup(this, 'asg', {
            instanceType: props?.instanceType,
            machineImage: ec2.MachineImage.latestAmazonLinux({
                cpuType: ec2.AmazonLinuxCpuType.ARM_64,
                generation: ec2.AmazonLinuxGeneration.AMAZON_LINUX_2,
            }),
            vpc: k3sVpc,
            securityGroup: k3sSecurityGroup,
            blockDevices: [
                {deviceName: '/dev/sdb', volume: autoscaling.BlockDeviceVolume.ebs(64)},
            ],
            maxCapacity: props?.instanceCount,
            minCapacity: props?.instanceCount,
            keyName: props?.keyName,
        });

        // setup the k3s token secret
        // this is used to securely share the k3s cluster token between nodes
        const k3sToken = new secretsmanager.Secret(this, 'token');

        k3sToken.grantRead(k3sAutoScalingGroup.role);
        k3sToken.grantWrite(k3sAutoScalingGroup.role);

        // add the instance user data
        // this is a bash script that is run on each instances inital start up
        k3sAutoScalingGroup.addUserData(
`
#!/bin/bash -xe

set -o pipefail

exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1

export LOGFILE=/var/log/k3s.log
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

REGION=${props?.env?.region}
TOKEN_SECRET_ID=${k3sToken.secretArn}

# ----

put_k3s_token () {
    aws secretsmanager put-secret-value --region $REGION --secret-id $TOKEN_SECRET_ID --secret-string file:///var/lib/rancher/k3s/server/node-token
}

get_k3s_token () {
    TOKEN=$(aws secretsmanager get-secret-value --region $REGION --secret-id $TOKEN_SECRET_ID --query 'SecretString' --output text)
    if [[ $? -ne 0 || "$TOKEN" != *":server:"* ]]; then
        return 1
    fi
}

get_oldest_node () {
    local INSTANCE; INSTANCE=$(
        aws ec2 describe-instances \
            --region $REGION \
            --filters \
                Name=tag:aws:cloudformation:stack-name,Values=${this.stackName} \
                Name=instance-state-name,Values=running \
            --query 'sort_by(Reservations[].Instances[], &LaunchTime)[*].[PrivateIpAddress,InstanceId]' \
            --output text | head -n1
    );

    if [[ $? -ne 0  || "$INSTANCE" == "None" ]]; then
        return 1
    fi

    OLDEST_IP=$(echo $INSTANCE | cut -d " " -f 1)
    OLDEST_ID=$(echo $INSTANCE | cut -d " " -f 2)
}

# ----

init_k3s_cluster () {
    echo "assuming node0 role... creating cluster"

    # start cluster init process
    # this will generate the cluster token if required

    local HAS_TOKEN=$(get_k3s_token; echo $?)

    if [ "$HAS_TOKEN" == "0" ]; then 
        INSTALL_ARGS="$INSTALL_ARGS --token $TOKEN"
    fi

    curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=stable INSTALL_K3S_EXEC="$INSTALL_ARGS --cluster-init" sh -s -
   
    if [ "$HAS_TOKEN" != "0" ]; then 
        put_k3s_token
    fi
}

join_k3s_cluster () {
    echo "awaiting node0... "

    # wait for the initial node to init the cluster
    # this waits for the token to be initalized; or fails after 5 minutes
    
    until get_k3s_token ; do
        echo "waiting for secret..."
        ((c++)) && ((c==60)) && c=0 && exit 1; sleep 5
    done

    curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=stable INSTALL_K3S_EXEC="$INSTALL_ARGS --server https://$OLDEST_IP:6443 --token $TOKEN" sh -s -
}

# ----

init_variables () {
    MY_ID=$(curl http://169.254.169.254/latest/meta-data/instance-id)
    MY_LOCAL_IP=$(curl http://169.254.169.254/latest/meta-data/local-ipv4)
    MY_PUBLIC_IP=$(curl http://169.254.169.254/latest/meta-data/public-ipv4)
    PROVIDER_ID=$(curl http://169.254.169.254/latest/meta-data/placement/availability-zone)/$MY_ID
    
    # get the instance id anb ip address of the oldest
    # ec2 instance with tag '${this.stackName}'
    
    until get_oldest_node ; do
        echo "failed to get oldest node... retrying"
        ((c++)) && ((c==60)) && c=0 && exit 1; sleep 5
    done

    echo "my_id    : $MY_ID"
    echo "oldest_id: $OLDEST_ID"

    INSTALL_ARGS="server \
        --node-ip $MY_LOCAL_IP \
        --node-external-ip $MY_PUBLIC_IP \
        --advertise-address $MY_LOCAL_IP \
        --write-kubeconfig-mode=0644 \
        --kubelet-arg=\"provider-id=aws:///$PROVIDER_ID\""
}

init_host () {
    CUR_HOSTNAME=$(cat /etc/hostname)

    hostnamectl set-hostname $MY_ID
    hostname $MY_ID

    sed -i "s/$CUR_HOSTNAME/$MY_ID/g" /etc/hosts
    sed -i "s/$CUR_HOSTNAME/$MY_ID/g" /etc/hostname
}

init_k3s () {
    if [ "$MY_ID" != "$OLDEST_ID" ]; then 
        join_k3s_cluster
    else
        init_k3s_cluster
    fi
}

# ----
# if k3s is installed the skip the rest of the script

if [ -x "$(command -v k3s)" ]; then
    echo "k3s is already installed"
    exit 0
fi

{
    init_variables
    init_host
    init_k3s
}
`);

        // allow route53 dns changes
        // TODO: use kube2iam to prevent rogue containers from assuming the role
        // https://github.com/kubernetes-sigs/external-dns/blob/master/docs/tutorials/aws.md
        k3sAutoScalingGroup.role.attachInlinePolicy(
            new iam.Policy(this, 'k3s-external-dns-policy', {document: AWS_ROUTE53_POLICY})
        );

        // set up the default instance role
        // this give the instances access to the management bucket and elb target health, which
        // is needed for initial cluster coordination
        k3sAutoScalingGroup.role.attachInlinePolicy(
            new iam.Policy(this, 'k3s-instance-policy', {
                statements: [
                    new iam.PolicyStatement({
                        resources: ['*'],
                        actions: [
                            'ec2:DescribeInstances',
                        ],
                    }),
                ]
            })
        );
    }
}
