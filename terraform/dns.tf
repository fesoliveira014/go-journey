# DNS configuration — only created when domain_name is provided.
# If you don't own a domain, leave domain_name empty and skip this file.

data "aws_route53_zone" "main" {
  count = var.domain_name != "" ? 1 : 0

  name         = var.domain_name
  private_zone = false
}

# Look up the ALB created by the AWS Load Balancer Controller.
# The LB is created asynchronously after the Ingress resource is applied.
data "aws_lb" "ingress" {
  count = var.domain_name != "" ? 1 : 0

  tags = {
    "elbv2.k8s.aws/cluster" = var.cluster_name
  }
}

# Alias record pointing the domain to the ALB
resource "aws_route53_record" "app" {
  count = var.domain_name != "" ? 1 : 0

  zone_id = data.aws_route53_zone.main[0].zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = data.aws_lb.ingress[0].dns_name
    zone_id                = data.aws_lb.ingress[0].zone_id
    evaluate_target_health = true
  }
}
