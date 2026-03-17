include "root" {
  path = find_in_parent_folders()
}

inputs = {
  cluster_name        = "prod"
  ami_release_version = "1.29.3-20240531"
}
