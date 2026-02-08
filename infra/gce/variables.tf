variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-east1"
}

variable "zone" {
  description = "GCP zone"
  type        = string
  default     = "us-east1-b"
}

variable "machine_type" {
  description = "GCE machine type (needs >= 8GB RAM for inference services)"
  type        = string
  default     = "e2-standard-4"
}

variable "disk_size_gb" {
  description = "Boot disk size in GB (needs space for Docker images with ML models)"
  type        = number
  default     = 50
}

variable "repo_url" {
  description = "Git repository URL (HTTPS)"
  type        = string
  default     = "https://github.com/RenatoCabral2022/WhatsWebService.git"
}

variable "repo_branch" {
  description = "Git branch to deploy"
  type        = string
  default     = "main"
}

variable "git_token" {
  description = "GitHub personal access token for private repo (leave empty for public)"
  type        = string
  default     = ""
  sensitive   = true
}
