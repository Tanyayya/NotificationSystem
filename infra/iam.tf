# Learner Lab provides a pre-created LabRole that has the permissions
# needed for ECS to pull from ECR and write to CloudWatch.
# We reference it by ARN rather than creating a new role —
# Learner Lab does not allow creating custom IAM roles.

data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# CloudWatch Log Groups — one per service
# ECS tasks write stdout/stderr here, visible in the AWS Console
resource "aws_cloudwatch_log_group" "gateway" {
  name              = "/ecs/${var.project_name}/gateway"
  retention_in_days = 7 # keep logs for 7 days to avoid storage costs

  tags = {
    Project = var.project_name
  }
}

resource "aws_cloudwatch_log_group" "ingestion" {
  name              = "/ecs/${var.project_name}/ingestion"
  retention_in_days = 7

  tags = {
    Project = var.project_name
  }
}

resource "aws_cloudwatch_log_group" "fanout" {
  name              = "/ecs/${var.project_name}/fanout"
  retention_in_days = 7

  tags = {
    Project = var.project_name
  }
}