# Kafka + Zookeeper running as ECS tasks instead of MSK
# MSK is not available in Learner Lab — this achieves the same result
# using the official Confluent Kafka Docker images on ECS Fargate

# CloudWatch log group for Kafka and Zookeeper
resource "aws_cloudwatch_log_group" "kafka" {
  name              = "/ecs/${var.project_name}/kafka"
  retention_in_days = 7

  tags = {
    Project = var.project_name
  }
}

# Security group for Zookeeper — only Kafka needs to talk to it
resource "aws_security_group" "zookeeper" {
  name        = "${var.project_name}-zookeeper-sg"
  description = "Allow Kafka to connect to Zookeeper"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Zookeeper port from Kafka"
    from_port       = 2181
    to_port         = 2181
    protocol        = "tcp"
    security_groups = [aws_security_group.kafka.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name    = "${var.project_name}-zookeeper-sg"
    Project = var.project_name
  }
}

# Security group for Kafka — ingestion and fanout connect here on port 9092
resource "aws_security_group" "kafka" {
  name        = "${var.project_name}-kafka-sg"
  description = "Allow Kafka traffic from ingestion and fanout services"
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
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name    = "${var.project_name}-kafka-sg"
    Project = var.project_name
  }
}

# Zookeeper task definition — required by Kafka for cluster coordination
resource "aws_ecs_task_definition" "zookeeper" {
  family                   = "${var.project_name}-zookeeper"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([
    {
      name  = "zookeeper"
      image = "confluentinc/cp-zookeeper:7.5.0"

      environment = [
        { name = "ZOOKEEPER_CLIENT_PORT", value = "2181" },
        { name = "ZOOKEEPER_TICK_TIME", value = "2000" }
      ]

      portMappings = [
        { containerPort = 2181, protocol = "tcp" }
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.kafka.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "zookeeper"
        }
      }

      essential = true
    }
  ])

  tags = {
    Project = var.project_name
  }
}

# Zookeeper ECS service — keeps one Zookeeper task running
resource "aws_ecs_service" "zookeeper" {
  name            = "${var.project_name}-zookeeper"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.zookeeper.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    security_groups  = [aws_security_group.zookeeper.id]
    assign_public_ip = true
  }

  tags = {
    Project = var.project_name
  }
}

# Kafka task definition — connects to Zookeeper via ECS service discovery
resource "aws_ecs_task_definition" "kafka" {
  family                   = "${var.project_name}-kafka"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 512
  memory                   = 1024
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([
    {
      name  = "kafka"
      image = "confluentinc/cp-kafka:7.5.0"

      environment = [
        { name = "KAFKA_BROKER_ID", value = "1" },
        # Zookeeper address — update this with the actual Zookeeper task IP after deploy
        { name = "KAFKA_ZOOKEEPER_CONNECT", value = "ZOOKEEPER_TASK_IP:2181" },
        # Advertised listeners — update with the actual Kafka task IP after deploy
        { name = "KAFKA_ADVERTISED_LISTENERS", value = "PLAINTEXT://KAFKA_TASK_IP:9092" },
        { name = "KAFKA_LISTENER_SECURITY_PROTOCOL_MAP", value = "PLAINTEXT:PLAINTEXT" },
        { name = "KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR", value = "1" },
        { name = "KAFKA_AUTO_CREATE_TOPICS_ENABLE", value = "true" }
      ]

      portMappings = [
        { containerPort = 9092, protocol = "tcp" }
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.kafka.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "kafka"
        }
      }

      essential = true
    }
  ])

  tags = {
    Project = var.project_name
  }
}

# Kafka ECS service
resource "aws_ecs_service" "kafka" {
  name            = "${var.project_name}-kafka"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.kafka.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    security_groups  = [aws_security_group.kafka.id]
    assign_public_ip = true
  }

  depends_on = [aws_ecs_service.zookeeper]

  tags = {
    Project = var.project_name
  }
}