variable "env" {
  description = "Environment identifier used as a prefix of resource names."
  type        = string
}

variable "slack_channel_id" {
  description = "ID of the Slack channel to post the notifications to (starts with C). The bot of the Slack app must be invited to it."
  type        = string

  validation {
    condition     = can(regex("^[CG][A-Z0-9]+$", var.slack_channel_id))
    error_message = "slack_channel_id must be a Slack channel ID (starting with C or G)."
  }
}
