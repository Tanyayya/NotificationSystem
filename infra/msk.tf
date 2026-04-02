# MSK Subnet Group — Kafka brokers need to know which subnets to live in
# Two subnets in different AZs required for MSK
resource "aws_msk_cluster" "main" {
  cluster_name           = "${var.project_name}-kafka"
  kafka_version          = "3.5.1"
  number_of_broker_nodes = 2 # one broker per AZ — minimum for MSK

  broker_node_group_info {
    instance_type  = var.kafka_instance_type # kafka.t3.small by default
    client_subnets = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    storage_info {
      ebs_storage_info {
        volume_size = 10 # 10GB per broker — sufficient for Week 1 testing
      }
    }
    security_groups = [aws_security_group.msk.id]
  }

  # Plaintext only — no TLS for simplicity in Learner Lab
  # In production you would enable TLS and SASL authentication
  client_authentication {
    unauthenticated = true
  }

  encryption_info {
    encryption_in_transit {
      client_broker = "PLAINTEXT"
      in_cluster    = false
    }
  }

  # CloudWatch metrics for Kafka broker monitoring
  # Useful for tracking consumer lag during experiments
  broker_logs {
    cloudwatch_logs {
      enabled   = true
      log_group = aws_cloudwatch_log_group.fanout.name
    }
  }

  open_monitoring {
    prometheus {
      jmx_exporter {
        enabled_in_broker = true
      }
      node_exporter {
        enabled_in_broker = true
      }
    }
  }

  tags = {
    Name    = "${var.project_name}-kafka"
    Project = var.project_name
  }
}