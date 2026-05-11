terraform {
  required_version = ">= 1.5.0"

  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "~> 6.0"
    }
  }

  # Terraform Cloud または OCI Object Storage をバックエンドにする場合はここを設定
  # backend "s3" {
  #   endpoint = "https://<namespace>.compat.objectstorage.ap-tokyo-1.oraclecloud.com"
  #   bucket   = "soc-app-tfstate"
  #   key      = "prod/terraform.tfstate"
  #   region   = "ap-tokyo-1"
  #   skip_region_validation      = true
  #   skip_credentials_validation = true
  #   skip_metadata_api_check     = true
  #   force_path_style            = true
  # }
}

locals {
  compartment_id = var.compartment_id != "" ? var.compartment_id : var.tenancy_ocid
}

provider "oci" {
  tenancy_ocid = var.tenancy_ocid
  user_ocid    = var.user_ocid
  fingerprint  = var.fingerprint
  region       = var.region

  # ローカル実行時は private_key_path、CI/CD では private_key_content を使用
  private_key      = var.private_key_content != "" ? var.private_key_content : null
  private_key_path = var.private_key_content == "" ? var.private_key_path : null
}

module "network" {
  source = "../../modules/oci-network"

  project_name        = var.project_name
  compartment_id      = local.compartment_id
  vcn_cidr            = var.vcn_cidr
  public_subnet_cidr  = var.public_subnet_cidr
  private_subnet_cidr = var.private_subnet_cidr
}

module "compute" {
  source = "../../modules/oci-compute"

  project_name        = var.project_name
  compartment_id      = local.compartment_id
  availability_domain = var.availability_domain
  subnet_id           = module.network.public_subnet_id
  ssh_authorized_keys = var.ssh_authorized_keys
  image_id            = var.image_id
}

module "database" {
  source = "../../modules/oci-database"

  project_name        = var.project_name
  compartment_id      = local.compartment_id
  availability_domain = var.availability_domain
  subnet_id           = module.network.private_subnet_id
  db_admin_password   = var.db_admin_password
}

module "storage" {
  source = "../../modules/oci-storage"

  project_name   = var.project_name
  compartment_id = local.compartment_id
  namespace      = var.storage_namespace
}
