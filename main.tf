variable "name" {
  default = "bastrd"
}

variable "az" {
  default = "us-east-1a"
}

variable "env" {}

variable "size" {
  default = "t3.nano"
}

variable "region" {
  default = "us-east-1"
}

variable "ssh_allowed_cidrs" {
  type = "list"

  default = [
    "0.0.0.0/0",
  ]
}

variable "ssh_group_name" {
  default = "bastrd"
}

variable "ssh_users" {
  type    = "list"
  default = []
}

variable "toolbox_image" {
  default = "docker.io/rochacon/bastrd-toolbox:latest"
}

provider "aws" {
  region = "${var.region}"
}

data "aws_caller_identity" "current" {}

data "aws_ami" "coreos" {
  most_recent = true

  filter {
    name   = "owner-id"
    values = ["595879546273"]
  }

  filter {
    name   = "name"
    values = ["CoreOS-stable-*"]
  }

  filter {
    name   = "state"
    values = ["available"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

data "aws_vpc" "vpc" {
  filter {
    name   = "tag:Env"
    values = ["${var.env}"]
  }
}

data "aws_subnet_ids" "public" {
  vpc_id = "${data.aws_vpc.vpc.id}"

  filter {
    name   = "availabilityZone"
    values = ["${var.az}"]
  }

  filter {
    name   = "tag:Tier"
    values = ["public"]
  }
}

resource "aws_security_group" "bastrd" {
  name        = "${var.env}-${var.name}-bastrd-sg"
  description = "${var.name} bastrd"

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["${var.ssh_allowed_cidrs}"]
  }

  lifecycle {
    create_before_destroy = true
  }

  tags {
    Name = "${var.env}-${var.name}-bastrd-sg"
    App  = "${var.name}-bastrd"
    Env  = "${var.env}"
  }

  vpc_id = "${data.aws_vpc.vpc.id}"
}

resource "aws_instance" "bastrd" {
  ami                    = "${data.aws_ami.coreos.id}"
  instance_type          = "${var.size}"
  iam_instance_profile   = "${aws_iam_instance_profile.bastrd.id}"
  subnet_id              = "${data.aws_subnet_ids.public.ids[0]}"
  user_data              = "${data.ignition_config.userdata.rendered}"
  vpc_security_group_ids = ["${aws_security_group.bastrd.id}"]

  // FIXME remove this
  key_name = "${var.name}"

  tags {
    Name = "${var.env}-${var.name}-bastrd"
    App  = "${var.name}-bastrd"
    Env  = "${var.env}"
  }
}

resource "aws_iam_role" "bastrd" {
  name = "${var.env}-bastrd"
  path = "/"

  assume_role_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "201812221130",
            "Effect": "Allow",
            "Action": "sts:AssumeRole",
            "Principal": {"Service":"ec2.amazonaws.com"}
        }
    ]
}
EOF
}

resource "aws_iam_role_policy" "bastrd" {
  name   = "${var.env}-bastrd"
  role   = "${aws_iam_role.bastrd.id}"
  policy = "${data.template_file.bastrd_policy.rendered}"
}

data "template_file" "bastrd_policy" {
  template = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "iam",
      "Effect": "Allow",
      "Action": [
        "iam:GetGroup",
        "iam:GetSSHPublicKey",
        "iam:ListAccessKeys",
        "iam:ListGroupsForUser",
        "iam:ListSSHPublicKeys",
        "sts:GetCallerIdentity"
      ],
      "Resource": ["*"]
    }
  ]
}
EOF
}

resource "aws_iam_instance_profile" "bastrd" {
  name = "${var.env}-bastrd"
  role = "${aws_iam_role.bastrd.name}"
}

output "ip" {
  value = "${aws_instance.bastrd.public_ip}"
}

output "private_ip" {
  value = "${aws_instance.bastrd.private_ip}"
}
