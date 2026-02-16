resource "google_compute_network" "whats" {
  name                    = "whats-network"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "whats" {
  name          = "whats-subnet"
  ip_cidr_range = "10.0.1.0/24"
  region        = var.region
  network       = google_compute_network.whats.id
}

# SSH access for debugging
resource "google_compute_firewall" "allow_ssh" {
  name    = "whats-allow-ssh"
  network = google_compute_network.whats.name

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["whats-vm"]
}

# Web client (nginx on port 80 + 443)
resource "google_compute_firewall" "allow_http" {
  name    = "whats-allow-http"
  network = google_compute_network.whats.name

  allow {
    protocol = "tcp"
    ports    = ["80", "443"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["whats-vm"]
}

# Control-plane REST API (direct access for testing)
resource "google_compute_firewall" "allow_control_plane" {
  name    = "whats-allow-control-plane"
  network = google_compute_network.whats.name

  allow {
    protocol = "tcp"
    ports    = ["8080"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["whats-vm"]
}

# WebRTC signaling (TCP) + media (UDP) on port 9090
resource "google_compute_firewall" "allow_webrtc" {
  name    = "whats-allow-webrtc"
  network = google_compute_network.whats.name

  allow {
    protocol = "tcp"
    ports    = ["9090"]
  }

  allow {
    protocol = "udp"
    ports    = ["9090"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["whats-vm"]
}

# WebRTC ICE ephemeral UDP ports
# Pion uses the OS ephemeral range since no SettingEngine port range is configured.
# For production, constrain via SettingEngine.SetEphemeralUDPPortRange() and tighten this rule.
resource "google_compute_firewall" "allow_webrtc_ice" {
  name    = "whats-allow-webrtc-ice"
  network = google_compute_network.whats.name

  allow {
    protocol = "udp"
    ports    = ["32768-65535"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["whats-vm"]
}

# Prometheus metrics endpoint
resource "google_compute_firewall" "allow_metrics" {
  name    = "whats-allow-metrics"
  network = google_compute_network.whats.name

  allow {
    protocol = "tcp"
    ports    = ["9092"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["whats-vm"]
}
