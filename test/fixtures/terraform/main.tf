resource "aws_eks_node_group" "workers" {
  cluster_name    = "prod"
  node_group_name = "workers"
  ami_release_version = "1.29.3-20240531"
  instance_types  = ["m5.xlarge"]
}
