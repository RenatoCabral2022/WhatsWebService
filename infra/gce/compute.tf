# Static external IP
resource "google_compute_address" "whats" {
  name   = "whats-service-ip"
  region = var.region
}

# Minimal service account (logging + monitoring only)
resource "google_service_account" "whats" {
  account_id   = "whats-service-vm"
  display_name = "Whats Service VM"
}

resource "google_project_iam_member" "logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.whats.email}"
}

resource "google_project_iam_member" "monitoring" {
  project = var.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.whats.email}"
}

# GCE instance
resource "google_compute_instance" "whats" {
  name         = "whats-service"
  machine_type = var.machine_type
  zone         = var.zone
  tags         = ["whats-vm"]

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2204-lts"
      size  = var.disk_size_gb
      type  = "pd-ssd"
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.whats.id
    access_config {
      nat_ip = google_compute_address.whats.address
    }
  }

  service_account {
    email  = google_service_account.whats.email
    scopes = ["logging-write", "monitoring-write"]
  }

  metadata = {
    startup-script = templatefile("${path.module}/startup.sh", {
      repo_url    = var.repo_url
      repo_branch = var.repo_branch
      git_token   = var.git_token
      public_ip   = google_compute_address.whats.address
    })
  }

  allow_stopping_for_update = true
}
