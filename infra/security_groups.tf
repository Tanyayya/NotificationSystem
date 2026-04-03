# ALB Security Group — accepts inbound HTTP and WebSocket traffic from the internet
resource "aws_security_group" "alb" {
  name        = "${var.project_name}-alb-sg"
  description = "Allow inbound HTTP and WebSocket traffic to the ALB"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP from internet"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "WebSocket / HTTP from internet on 8080"
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name    = "${var.project_name}-alb-sg"
    Project = var.project_name
  }
}

# Gateway Security Group — accepts traffic from the ALB only
resource "aws_security_group" "gateway" {
  name        = "${var.project_name}-gateway-sg"
  description = "Allow inbound traffic to gateway from ALB only"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "WebSocket traffic from ALB"
    from_port       = var.gateway_port
    to_port         = var.gateway_port
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name    = "${var.project_name}-gateway-sg"
    Project = var.project_name
  }
}

# Ingestion Security Group — accepts traffic from the internet (REST API)
resource "aws_security_group" "ingestion" {
  name        = "${var.project_name}-ingestion-sg"
  description = "Allow inbound HTTP traffic to ingestion API"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP from internet"
    from_port   = var.ingestion_port
    to_port     = var.ingestion_port
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name    = "${var.project_name}-ingestion-sg"
    Project = var.project_name
  }
}

# Fan-out Worker Security Group — no inbound needed, only outbound to Kafka and Redis
resource "aws_security_group" "fanout" {
  name        = "${var.project_name}-fanout-sg"
  description = "Fan-out worker - outbound only to Kafka and Redis"
  vpc_id      = aws_vpc.main.id

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# MSK Security Group — accepts Kafka traffic from ingestion and fanout only
resource "aws_security_group" "msk" {
  name        = "${var.project_name}-msk-sg"
  description = "Allow inbound Kafka traffic from ingestion and fanout only"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Kafka from ingestion API"
    from_port       = 9092
    to_port         = 9092
    protocol        = "tcp"
    security_groups = [aws_security_group.ingestion.id]
  }

  ingress {
    description     = "Kafka from fanout worker"
    from_port       = 9092
    to_port         = 9092
    protocol        = "tcp"
    security_groups = [aws_security_group.fanout.id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name    = "${var.project_name}-msk-sg"
    Project = var.project_name
  }
}

# Redis Security Group — accepts traffic from gateway and fan-out worker only
resource "aws_security_group" "redis" {
  name        = "${var.project_name}-redis-sg"
  description = "Allow inbound Redis traffic from gateway and fanout only"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Redis from gateway"
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [aws_security_group.gateway.id]
  }

  ingress {
    description     = "Redis from fanout worker"
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [aws_security_group.fanout.id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name    = "${var.project_name}-redis-sg"
    Project = var.project_name
  }
}