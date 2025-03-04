package knowledgebase

import (
	"fmt"

	"github.com/klothoplatform/klotho/pkg/construct"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledge_base"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources"
	k8sResources "github.com/klothoplatform/klotho/pkg/provider/kubernetes/resources"
)

var IamKB = knowledgebase.Build(
	knowledgebase.EdgeBuilder[*resources.RdsProxy, *resources.IamRole]{
		Configure: func(proxy *resources.RdsProxy, role *resources.IamRole, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			role.AssumeRolePolicyDoc = resources.RDS_ASSUME_ROLE_POLICY
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.IamRole, *resources.Secret]{
		Configure: func(role *resources.IamRole, secret *resources.Secret, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			secretPolicyDoc := resources.CreateAllowPolicyDocument([]string{"secretsmanager:DescribeSecret", "secretsmanager:GetSecretValue"}, []construct.IaCValue{{ResourceId: secret.Id(), Property: resources.ARN_IAC_VALUE}})
			inlinePol := resources.NewIamInlinePolicy(fmt.Sprintf("%s-secretpolicy", secret.Name), role.ConstructRefs.CloneWith(secret.ConstructRefs), secretPolicyDoc)
			role.InlinePolicies = append(role.InlinePolicies, inlinePol)
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.IamPolicy, *resources.Secret]{
		Configure: func(policy *resources.IamPolicy, secret *resources.Secret, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			policy.AddPolicyDocument(resources.CreateAllowPolicyDocument([]string{"secretsmanager:DescribeSecret", "secretsmanager:GetSecretValue"}, []construct.IaCValue{{ResourceId: secret.Id(), Property: resources.ARN_IAC_VALUE}}))
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.IamRole, *resources.IamPolicy]{
		Configure: func(role *resources.IamRole, policy *resources.IamPolicy, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			role.AddManagedPolicy(construct.IaCValue{ResourceId: policy.Id(), Property: resources.ARN_IAC_VALUE})
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.LambdaFunction, *resources.IamRole]{
		Configure: func(lambda *resources.LambdaFunction, role *resources.IamRole, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			role.AssumeRolePolicyDoc = resources.LAMBDA_ASSUMER_ROLE_POLICY
			if len(lambda.Subnets) == 0 {
				role.AddAwsManagedPolicies([]string{"arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"})
			} else {
				role.AddAwsManagedPolicies([]string{"arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"})
			}
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.EcsTaskDefinition, *resources.IamRole]{
		Configure: func(taskDef *resources.EcsTaskDefinition, role *resources.IamRole, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			role.AssumeRolePolicyDoc = resources.ECS_ASSUMER_ROLE_POLICY
			role.AddAwsManagedPolicies([]string{"arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"})
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.AppRunnerService, *resources.IamRole]{
		Configure: func(service *resources.AppRunnerService, role *resources.IamRole, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			var APP_RUNNER_ASSUMER_ROLE_POLICY = &resources.PolicyDocument{
				Version: resources.VERSION,
				Statement: []resources.StatementEntry{
					{
						Action: []string{"sts:AssumeRole"},
						Principal: &resources.Principal{
							Service: "build.apprunner.amazonaws.com",
						},
						Effect: "Allow",
					},
					{
						Action: []string{"sts:AssumeRole"},
						Principal: &resources.Principal{
							Service: "tasks.apprunner.amazonaws.com",
						},
						Effect: "Allow",
					},
				},
			}
			role.AssumeRolePolicyDoc = APP_RUNNER_ASSUMER_ROLE_POLICY
			role.AddAwsManagedPolicies([]string{"arn:aws:iam::aws:policy/service-role/AWSAppRunnerServicePolicyForECRAccess"})
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.IamRole, *resources.DynamodbTable]{
		Configure: func(role *resources.IamRole, table *resources.DynamodbTable, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			actions := []string{"dynamodb:*"}
			policyResources := []construct.IaCValue{
				{ResourceId: table.Id(), Property: resources.ARN_IAC_VALUE},
				{ResourceId: table.Id(), Property: resources.DYNAMODB_TABLE_BACKUP_IAC_VALUE},
				{ResourceId: table.Id(), Property: resources.DYNAMODB_TABLE_INDEX_IAC_VALUE},
				{ResourceId: table.Id(), Property: resources.DYNAMODB_TABLE_EXPORT_IAC_VALUE},
				{ResourceId: table.Id(), Property: resources.DYNAMODB_TABLE_STREAM_IAC_VALUE},
			}
			doc := resources.CreateAllowPolicyDocument(actions, policyResources)
			inlinePol := resources.NewIamInlinePolicy(fmt.Sprintf("%s-dynamodb-policy", table.Name), role.ConstructRefs.CloneWith(table.ConstructRefs), doc)
			role.InlinePolicies = append(role.InlinePolicies, inlinePol)
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.EksCluster, *resources.IamRole]{
		Configure: func(cluster *resources.EksCluster, role *resources.IamRole, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			role.AssumeRolePolicyDoc = resources.EKS_ASSUME_ROLE_POLICY
			role.AddAwsManagedPolicies([]string{"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"})
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.EksFargateProfile, *resources.IamRole]{
		Configure: func(profile *resources.EksFargateProfile, role *resources.IamRole, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			role.AssumeRolePolicyDoc = resources.EKS_FARGATE_ASSUME_ROLE_POLICY
			role.AddAwsManagedPolicies([]string{
				"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
				"arn:aws:iam::aws:policy/AmazonEKSFargatePodExecutionRolePolicy",
			})
			role.InlinePolicies = append(role.InlinePolicies, resources.NewIamInlinePolicy("fargate-pod-execution-policy", profile.ConstructRefs,
				&resources.PolicyDocument{Version: resources.VERSION, Statement: []resources.StatementEntry{
					{
						Effect: "Allow",
						Action: []string{
							"logs:CreateLogStream",
							"logs:CreateLogGroup",
							"logs:DescribeLogStreams",
							"logs:PutLogEvents",
						},
						Resource: []construct.IaCValue{{Property: "*"}},
					},
				},
				}))
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.EksNodeGroup, *resources.IamRole]{
		Configure: func(cluster *resources.EksNodeGroup, role *resources.IamRole, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			role.AssumeRolePolicyDoc = resources.EC2_ASSUMER_ROLE_POLICY
			role.AddAwsManagedPolicies([]string{
				"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
				"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
				"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
				"arn:aws:iam::aws:policy/AWSCloudMapFullAccess",
				"arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy",
				"arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore",
			})
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.IamRole, *resources.OpenIdConnectProvider]{
		Configure: func(role *resources.IamRole, oidc *resources.OpenIdConnectProvider, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			if len(role.ConstructRefs) > 1 {
				return fmt.Errorf("iam role %s must only have one construct ref, but has %d, %s", role.Name, len(role.ConstructRefs), role.ConstructRefs)
			}

			var upstreamServiceAccount *k8sResources.ServiceAccount
			for _, res := range dag.GetUpstreamResources(role) {
				if sa, ok := res.(*k8sResources.ServiceAccount); ok {
					upstreamServiceAccount = sa
					break
				}
			}
			if upstreamServiceAccount != nil {
				role.AssumeRolePolicyDoc = resources.GetServiceAccountAssumeRolePolicy(upstreamServiceAccount.Object.Name, upstreamServiceAccount.Object.Namespace, oidc)
				return nil
			}

			var ref construct.ResourceId
			for cons := range role.ConstructRefs {
				ref = cons
			}
			role.AssumeRolePolicyDoc = resources.GetServiceAccountAssumeRolePolicy(ref.Name, ref.Namespace, oidc)
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.IamRole, *resources.S3Bucket]{
		Configure: func(role *resources.IamRole, bucket *resources.S3Bucket, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			role.InlinePolicies = append(role.InlinePolicies, resources.NewIamInlinePolicy(
				fmt.Sprintf(`%s-access`, bucket.Name),
				role.ConstructRefs.CloneWith(bucket.ConstructRefs),
				resources.CreateAllowPolicyDocument(
					[]string{"s3:*"},
					[]construct.IaCValue{
						{ResourceId: bucket.Id(), Property: resources.ARN_IAC_VALUE},
						{ResourceId: bucket.Id(), Property: resources.ALL_BUCKET_DIRECTORY_IAC_VALUE},
					})))
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.IamRole, *resources.SesEmailIdentity]{
		Configure: func(role *resources.IamRole, emailIdentity *resources.SesEmailIdentity, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			role.InlinePolicies = append(role.InlinePolicies, resources.NewIamInlinePolicy(
				fmt.Sprintf(`%s-access`, emailIdentity.Name),
				role.ConstructRefs.CloneWith(emailIdentity.ConstructRefs),
				resources.CreateAllowPolicyDocument(
					[]string{"ses:SendEmail", "ses:SendRawEmail"},
					[]construct.IaCValue{
						{ResourceId: emailIdentity.Id(), Property: resources.ARN_IAC_VALUE},
					})))
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.IamPolicy, *resources.LambdaFunction]{
		Configure: func(policy *resources.IamPolicy, function *resources.LambdaFunction, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			policy.AddPolicyDocument(resources.CreateAllowPolicyDocument([]string{"lambda:InvokeFunction"}, []construct.IaCValue{{ResourceId: function.Id(), Property: resources.ARN_IAC_VALUE}}))
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.IamRole, *resources.RdsInstance]{
		Configure: func(role *resources.IamRole, instance *resources.RdsInstance, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			inlinePol := resources.NewIamInlinePolicy(fmt.Sprintf("%s-connectionpolicy", instance.Name),
				role.ConstructRefs.CloneWith(instance.ConstructRefs), instance.GetConnectionPolicyDocument())
			role.InlinePolicies = append(role.InlinePolicies, inlinePol)
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.IamRole, *resources.RdsProxy]{},
	knowledgebase.EdgeBuilder[*resources.RolePolicyAttachment, *resources.IamRole]{
		Configure: func(policy *resources.RolePolicyAttachment, role *resources.IamRole, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			policy.Role = role
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.RolePolicyAttachment, *resources.IamPolicy]{
		Configure: func(policy *resources.RolePolicyAttachment, iamPolicy *resources.IamPolicy, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			policy.Policy = iamPolicy
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.IamPolicy, *resources.PrivateDnsNamespace]{
		Configure: func(policy *resources.IamPolicy, namespace *resources.PrivateDnsNamespace, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			policy.AddPolicyDocument(resources.CreateAllowPolicyDocument([]string{"servicediscovery:DiscoverInstances"}, []construct.IaCValue{{Property: construct.ALL_RESOURCES_IAC_VALUE}}))
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.InstanceProfile, *resources.IamRole]{
		Configure: func(source *resources.InstanceProfile, destination *resources.IamRole, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			inlinePolicy := resources.NewIamInlinePolicy(fmt.Sprintf("%s-instanceProfilePolicy", source.Name), source.ConstructRefs.CloneWith(destination.ConstructRefs),
				&resources.PolicyDocument{
					Version: resources.VERSION,
					Statement: resources.CreateAllowPolicyDocument([]string{
						"iam:ListInstanceProfiles",
						"ec2:Describe*",
						"ec2:Search*",
						"ec2:Get*",
					}, []construct.IaCValue{{Property: "*"}}).Statement,
				},
			)
			inlinePolicy.Policy.Statement = append(inlinePolicy.Policy.Statement, resources.StatementEntry{
				Effect:   "Allow",
				Action:   []string{"iam:PassRole"},
				Resource: []construct.IaCValue{{Property: "*"}},
				Condition: &resources.Condition{
					StringEquals: map[construct.IaCValue]string{
						{Property: "iam:PassedToService"}: "ec2.amazonaws.com",
					},
				},
			})
			destination.InlinePolicies = append(destination.InlinePolicies, inlinePolicy)
			return nil
		},
		DirectEdgeOnly: true,
	},
	knowledgebase.EdgeBuilder[*resources.IamRole, *resources.EfsMountTarget]{
		Configure: func(role *resources.IamRole, mountTarget *resources.EfsMountTarget, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			role.AddAwsManagedPolicies([]string{"arn:aws:iam::aws:policy/AmazonElasticFileSystemClientReadWriteAccess"})
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.IamRole, *resources.EfsAccessPoint]{
		Configure: func(role *resources.IamRole, accessPoint *resources.EfsAccessPoint, dag *construct.ResourceGraph, data knowledgebase.EdgeData) error {
			role.AddAwsManagedPolicies([]string{"arn:aws:iam::aws:policy/AmazonElasticFileSystemClientReadWriteAccess"})
			return nil
		},
	},
)
