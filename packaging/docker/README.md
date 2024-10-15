## Docker image build
This folder defines the dockerfile and its dependencies used to build the official SSM Agent container image.

The examples below require that you have the `aws-cli` and `jq` installed

### How to build agent image
The example below shows how to download an SSM Agent amd64 rpm from us-east-1 with version 3.1.90.0.
```shell
cd packaging/docker
wget https://amazon-ssm-us-east-1.s3.us-east-1.amazonaws.com/3.1.90.0/linux_amd64/amazon-ssm-agent.rpm
docker build -t amazon-ssm-agent:custom .
```

#### Before running image
The SSM Agent needs to be registered with SSM and for that the agent needs an activation object. The example below shows how to create an activation and use it in the run examples below. For requirements of iam-role and more information on hybrid activation feature of SSM, see [this page](https://docs.aws.amazon.com/systems-manager/latest/userguide/sysman-managed-instance-activation.html)
```shell
REGION=us-east-1
NUM_REGISTRATIONS=1
IAM_ROLE=SSMServiceRole
ACTIVATION=$(aws ssm create-activation --iam-role $IAM_ROLE --registration-limit $NUM_REGISTRATIONS --tags Key=Name,Value=ContainerAgent --region $REGION --output json)
echo $ACTIVATION | jq --arg region $REGION '. + {Region: $region}' > user-data
```

### How to run custom agent image build locally
```shell
docker run -d -v `pwd`/user-data:/.ssm/containers/current/user-data amazon-ssm-agent:custom
```

### How to run public agent image
```shell
docker run -d -v `pwd`/user-data:/.ssm/containers/current/user-data public.ecr.aws/amazon-ssm-agent/amazon-ssm-agent
```

### Get the SSM Agent instance id in a running container
```shell
docker logs <container-id> | grep --color=never "SSM Agent instance id is"
```
