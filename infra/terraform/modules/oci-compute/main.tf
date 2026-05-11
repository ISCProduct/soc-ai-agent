resource "oci_core_instance" "app" {
  compartment_id      = var.compartment_id
  availability_domain = var.availability_domain
  display_name        = "${var.project_name}-instance"
  # fault_domain未指定でOCIに自動選択させる（指定すると容量不足になりやすい）
  shape = var.shape
  shape_config {
    ocpus         = var.ocpus
    memory_in_gbs = var.memory_in_gbs
  }

  source_details {
    source_type = "image"
    source_id   = var.image_id
  }

  create_vnic_details {
    subnet_id        = var.subnet_id
    display_name     = "${var.project_name}-vnic"
    assign_public_ip = true
  }

  metadata = {
    ssh_authorized_keys = var.ssh_authorized_keys
    user_data           = base64encode(templatefile("${path.module}/cloud-init.sh", {
      project_name = var.project_name
    }))
  }
}
