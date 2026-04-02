module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.5"

  name = "${var.project_name}-vpc"
  cidr = var.vpc_cidr

  azs             = slice(data.aws_availability_zones.available.names, 0, 2)
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway   = true
  single_nat_gateway   = true
  enable_dns_hostnames = true
  enable_dns_support   = true

  public_subnet_tags = {
    "kubernetes.io/role/elb"                    = 1
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb"           = 1
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
  }
}

# Security group for RDS — allows inbound PostgreSQL from EKS nodes only
resource "aws_security_group" "rds" {
  name_prefix = "${var.project_name}-rds-"
  description = "Allow PostgreSQL access from EKS nodes"
  vpc_id      = module.vpc.vpc_id

  tags = { Name = "${var.project_name}-rds" }
}

resource "aws_security_group_rule" "rds_ingress" {
  type                     = "ingress"
  from_port                = 5432
  to_port                  = 5432
  protocol                 = "tcp"
  security_group_id        = aws_security_group.rds.id
  source_security_group_id = module.eks.node_security_group_id
  description              = "PostgreSQL from EKS nodes"
}

# Security group for MSK — allows inbound Kafka from EKS nodes only
resource "aws_security_group" "msk" {
  name_prefix = "${var.project_name}-msk-"
  description = "Allow Kafka access from EKS nodes"
  vpc_id      = module.vpc.vpc_id

  tags = { Name = "${var.project_name}-msk" }
}

resource "aws_security_group_rule" "msk_ingress_plaintext" {
  type                     = "ingress"
  from_port                = 9092
  to_port                  = 9092
  protocol                 = "tcp"
  security_group_id        = aws_security_group.msk.id
  source_security_group_id = module.eks.node_security_group_id
  description              = "Kafka plaintext from EKS nodes"
}
