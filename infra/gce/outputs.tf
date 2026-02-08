output "public_ip" {
  description = "VM public IP address"
  value       = google_compute_address.whats.address
}

output "web_client_url" {
  description = "Web client URL"
  value       = "http://${google_compute_address.whats.address}"
}

output "control_plane_url" {
  description = "Control plane API URL (direct)"
  value       = "http://${google_compute_address.whats.address}:8080"
}

output "metrics_url" {
  description = "Prometheus metrics URL"
  value       = "http://${google_compute_address.whats.address}:9092/metrics"
}

output "ssh_command" {
  description = "SSH command to connect to the VM"
  value       = "gcloud compute ssh whats-service --zone=${var.zone} --project=${var.project_id}"
}
