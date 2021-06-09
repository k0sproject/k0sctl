output "controller-instances"{
  description = "List of IDs of controller instances"
  value       = aws_instance.cluster-controller
}

output "worker-instances"{
  description = "List of IDs of worker instances"
  value       = aws_instance.cluster-workers
}

output "cluster-security_group"{
    value = aws_security_group.cluster_allow_ssh.id
}