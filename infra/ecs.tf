# ECS Cluster — logical grouping for all three services
resource "aws_ecs_cluster" "main" {
  name = "${var.project_name}-cluster"

  # Enable CloudWatch Container Insights for CPU/memory metrics per task
  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  tags = {
    Name    = "${var.project_name}-cluster"
    Project = var.project_name
  }
}

# ─────────────────────────────────────────────
# GATEWAY SERVICE
# ─────────────────────────────────────────────

resource "aws_ecs_task_definition" "gateway" {
  family                   = "${var.project_name}-gateway"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc" # required for Fargate
  cpu                      = var.ecs_task_cpu
  memory                   = var.ecs_task_memory
  execution_role_arn       = data.aws_iam_role.lab_role.arn # pulls images from ECR
  task_role_arn            = data.aws_iam_role.lab_role.arn # runtime permissions

  container_definitions = jsonencode([
    {
      name  = "gateway"
      image = var.gateway_image != "" ? var.gateway_image : "${aws_ecr_repository.gateway.repository_url}:latest"

      portMappings = [
        {
          containerPort = var.gateway_port
          protocol      = "tcp"
        }
      ]

      environment = [
        # Redis endpoint injected at deploy time from ElastiCache output
        { name = "REDIS_ADDR", value = "${aws_elasticache_cluster.redis.cache_nodes[0].address}:6379" },
        # TASK_ID helps the gateway identify itself in Redis registration
        # ECS_CONTAINER_METADATA_URI_V4 is auto-injected by Fargate
        { name = "TASK_ID", value = "ecs-gateway" }
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.gateway.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "gateway"
        }
      }

      essential = true
    }
  ])

  tags = {
    Project = var.project_name
  }
}

resource "aws_ecs_service" "gateway" {
  name            = "${var.project_name}-gateway"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.gateway.arn
  desired_count   = 1 # start with 1 task, scale up for experiments
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    security_groups  = [aws_security_group.gateway.id]
    assign_public_ip = true # required since we're using public subnets without NAT
  }

  # Register gateway tasks with the ALB target group
  load_balancer {
    target_group_arn = aws_lb_target_group.gateway.arn
    container_name   = "gateway"
    container_port   = var.gateway_port
  }

  # Wait for ALB to be ready before starting the service
  depends_on = [aws_lb_listener.gateway]

  tags = {
    Project = var.project_name
  }
}

# ─────────────────────────────────────────────
# INGESTION SERVICE
# ─────────────────────────────────────────────

resource "aws_ecs_task_definition" "ingestion" {
  family                   = "${var.project_name}-ingestion"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.ecs_task_cpu
  memory                   = var.ecs_task_memory
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([
    {
      name  = "ingestion"
      image = var.ingestion_image != "" ? var.ingestion_image : "${aws_ecr_repository.ingestion.repository_url}:latest"

      portMappings = [
        {
          containerPort = var.ingestion_port
          protocol      = "tcp"
        }
      ]

      environment = [
        # Kafka bootstrap brokers injected at deploy time from MSK output
        # Update KAFKA_BROKERS with the Kafka task IP after first deploy
        { name = "KAFKA_BROKERS", value = "${var.kafka_broker_ip}:9092" },
        { name = "KAFKA_TOPIC", value = "notification-events" }
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.ingestion.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "ingestion"
        }
      }

      essential = true
    }
  ])

  tags = {
    Project = var.project_name
  }
}

/* STEP 2 — uncomment after getting Kafka task IP
resource "aws_ecs_service" "ingestion" {
  name            = "${var.project_name}-ingestion"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.ingestion.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    security_groups  = [aws_security_group.ingestion.id]
    assign_public_ip = true
  }

  tags = {
    Project = var.project_name
  }
}
*/

# ─────────────────────────────────────────────
# FAN-OUT WORKER SERVICE
# ─────────────────────────────────────────────

resource "aws_ecs_task_definition" "fanout" {
  family                   = "${var.project_name}-fanout"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.ecs_task_cpu
  memory                   = var.ecs_task_memory
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([
    {
      name  = "fanout"
      image = var.fanout_image != "" ? var.fanout_image : "${aws_ecr_repository.fanout.repository_url}:latest"

      environment = [
        { name = "KAFKA_BROKERS", value = "${var.kafka_broker_ip}:9092" },
        { name = "KAFKA_TOPIC", value = "notification-events" },
        { name = "KAFKA_GROUP_ID", value = "fanout-consumer-group" },
        { name = "REDIS_ADDR", value = "${aws_elasticache_cluster.redis.cache_nodes[0].address}:6379" }
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.fanout.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "fanout"
        }
      }

      essential = true
    }
  ])

  tags = {
    Project = var.project_name
  }
}

/* STEP 2 — uncomment after getting Kafka task IP
resource "aws_ecs_service" "fanout" {
  name            = "${var.project_name}-fanout"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.fanout.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    security_groups  = [aws_security_group.fanout.id]
    assign_public_ip = true
  }

  tags = {
    Project = var.project_name
  }
}
*/