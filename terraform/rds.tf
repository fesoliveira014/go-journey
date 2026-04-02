locals {
  databases = {
    catalog     = "catalog"
    auth        = "auth"
    reservation = "reservation"
  }
}

resource "aws_db_subnet_group" "main" {
  name       = "${var.project_name}-db"
  subnet_ids = module.vpc.private_subnets

  tags = { Name = "${var.project_name}-db-subnet-group" }
}

resource "aws_db_instance" "databases" {
  for_each = local.databases

  identifier = "${var.project_name}-${each.key}"

  engine         = "postgres"
  engine_version = "16.4"
  instance_class = "db.t3.micro"

  allocated_storage = 20
  storage_type      = "gp3"

  db_name  = each.value
  username = "postgres"

  manage_master_user_password = true

  vpc_security_group_ids = [aws_security_group.rds.id]
  db_subnet_group_name   = aws_db_subnet_group.main.name

  skip_final_snapshot     = true # Learning project only — NEVER in production
  backup_retention_period = 0    # Disable backups to reduce cost — production default is 7

  tags = { Name = "${var.project_name}-${each.key}" }
}
