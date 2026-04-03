# Subnet group — tells ElastiCache which subnets it can live in
# Requires at least 2 subnets in different AZs
resource "aws_elasticache_subnet_group" "main" {
  name       = "${var.project_name}-redis-subnet-group"
  subnet_ids = [aws_subnet.public_1.id, aws_subnet.public_2.id]

  tags = {
    Project = var.project_name
  }
}

# Redis cluster — single node, no replication needed for Week 1 testing
resource "aws_elasticache_cluster" "redis" {
  cluster_id           = "${var.project_name}-redis"
  engine               = "redis"
  node_type            = var.redis_node_type # cache.t3.micro by default
  num_cache_nodes      = 1
  parameter_group_name = "default.redis7"
  engine_version       = "7.0"
  port                 = 6379
  subnet_group_name    = aws_elasticache_subnet_group.main.name
  security_group_ids   = [aws_security_group.redis.id]

  tags = {
    Name    = "${var.project_name}-redis"
    Project = var.project_name
  }
}