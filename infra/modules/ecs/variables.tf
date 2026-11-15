variable "env" {
  description = "Environment identifier used as a prefix of resource names."
  type        = string
}

variable "vpc_id" {
  description = "ID of the VPC to run the tasks in."
  type        = string
}

variable "vpc_cidr" {
  description = "CIDR block of the VPC. The tasks reach the interface VPC endpoints within it."
  type        = string
}

variable "private_subnet_ids" {
  description = "IDs of the private subnets to run the tasks in (one per availability zone)."
  type        = list(string)
}

variable "alb_security_group_id" {
  description = "ID of the security group of the ALB. Only it may reach the container port."
  type        = string
}

variable "alb_arn_suffix" {
  description = "ARN suffix of the ALB, used as the dimension of the 5xx alarm."
  type        = string
}

variable "target_group_arn" {
  description = "ARN of the target group that the service registers its tasks in for rolling deployments."
  type        = string
}

variable "image" {
  description = "Image of the api container for the first task definition, pinned by digest (<repository_url>@sha256:...)."
  type        = string
}

variable "container_port" {
  description = "Port that the application listens on."
  type        = number
  default     = 8080
}

variable "cpu" {
  description = "CPU units of a task (256 = 0.25 vCPU)."
  type        = number
  default     = 256
}

variable "memory" {
  description = "Memory of a task in MiB."
  type        = number
  default     = 512
}

variable "desired_count" {
  description = "Number of tasks to run initially. Auto scaling changes it afterwards."
  type        = number
  default     = 2
}

variable "max_count" {
  description = "Maximum number of tasks that auto scaling may run."
  type        = number
  default     = 4
}

variable "stop_timeout" {
  description = "Seconds to wait after SIGTERM before SIGKILL. Keep it longer than the shutdown timeout of the application."
  type        = number
  default     = 30
}

variable "health_check_grace_period" {
  description = "Seconds to ignore failing ALB health checks after a task starts."
  type        = number
  default     = 30
}

variable "log_retention_days" {
  description = "Days to keep the container logs."
  type        = number
  default     = 3
}

variable "target_5xx_threshold" {
  description = "Number of 5xx responses from the targets within a minute that triggers a rollback."
  type        = number
  default     = 5
}
