# MSK Cluster — managed Kafka on AWS
# Requires kafka:CreateCluster permission — available in sandbox but not Learner Lab

resource "aws_msk_cluster" "main" {
  cluster_name           = "${var.project_name}-kafka"
  kafka_version          = "3.5.1"
  number_of_broker_nodes = 2 # one broker per AZ

  broker_node_group_info {
    instance_type  = var.kafka_instance_type # kafka.t3.small by default
    client_subnets = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    storage_info {
      ebs_storage_info {
        volume_size = 10
      }
    }
    security_groups = [aws_security_group.msk.id]
  }

  client_authentication {
    unauthenticated = true
  }

  encryption_info {
    encryption_in_transit {
      client_broker = "PLAINTEXT"
      in_cluster    = false
    }
  }

  logging_info {
    broker_logs {
      cloudwatch_logs {
        enabled   = true
        log_group = aws_cloudwatch_log_group.fanout.name
      }
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