variable "env" {
  description = "Environment identifier used as a prefix of resource names."
  type        = string
}

variable "name" {
  description = "Name of the application. The repository is named <env>-amber-<name>."
  type        = string
}

variable "keep_image_count" {
  description = "Number of the most recent images to keep. Older images are expired by the lifecycle policy."
  type        = number
  default     = 5
}
