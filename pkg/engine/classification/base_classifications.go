package classification

var BaseClassificationDocument = &ClassificationDocument{
	Classifications: map[string]Classification{
		"aws:app_runner_service:":  {Gives: []Gives{}, Is: []string{"compute", "serverless"}},
		"aws:cloudfront_distribution:": {Gives: []Gives{}, Is: []string{"network", "cdn"}},
		"aws:ec2_instance:":            {Gives: []Gives{}, Is: []string{"compute", "instance"}},
		"aws:ecr_image:":               {Gives: []Gives{}, Is: []string{"container_image"}},
		"aws:ecs_cluster:":             {Gives: []Gives{}, Is: []string{"cluster"}},
		"aws:ecs_service:":             {Gives: []Gives{}, Is: []string{"compute"}},
		"aws:dynamodb_table:":          {Gives: []Gives{}, Is: []string{"storage", "kv", "nosql"}},
		"aws:efs_file_system:":         {Gives: []Gives{}, Is: []string{"storage", "filesystem"}},
		"aws:eks_cluster:":             {Gives: []Gives{}, Is: []string{"cluster", "kubernetes"}},
		"aws:elasticache_cluster:":     {Gives: []Gives{}, Is: []string{"storage", "redis", "cache"}},
		"aws:lambda_function:":         {Gives: []Gives{}, Is: []string{"compute", "serverless"}},
		"aws:load_balancer:":           {Gives: []Gives{}, Is: []string{"network", "loadbalancer"}},
		"aws:rds_instance:":            {Gives: []Gives{}, Is: []string{"storage", "relational"}},
		"aws:rds_proxy:":               {Gives: []Gives{}, Is: []string{"proxy"}},
		"aws:rest_api:":                {Gives: []Gives{}, Is: []string{"api"}},
		"aws:route53_hosted_zone:":     {Gives: []Gives{}, Is: []string{"network", "dns"}},
		"aws:s3_bucket:":               {Gives: []Gives{}, Is: []string{"storage", "blob"}},
		"aws:sns_topic:":               {Gives: []Gives{}, Is: []string{"messaging", "pubsub"}},
		"aws:sqs_queue:":               {Gives: []Gives{}, Is: []string{"messaging", "queue"}},
		"aws:secret:":                  {Gives: []Gives{}, Is: []string{"storage", "secret"}},
		"aws:vpc:":                     {Gives: []Gives{}, Is: []string{"network"}},
		"docker:image:":                {Gives: []Gives{}, Is: []string{"container_image"}},
		"kubernetes:deployment:":       {Gives: []Gives{}, Is: []string{"compute", "kubernetes"}},
		"kubernetes:helm_chart:":       {Gives: []Gives{}, Is: []string{"kubernetes"}},
		"kubernetes:pod:":              {Gives: []Gives{}, Is: []string{"compute", "kubernetes"}},
	},
}
