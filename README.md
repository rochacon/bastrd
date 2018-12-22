# bastrd - bastion server for secure environments

`bastrd` builds on top of the ideas behind [keymaker](https://github.com/kislyuk/keymaker/) and [toolbox](https://github.com/coreos/toolbox) to build a secure shared bastion server for restricted environments.

:warning: `bastrd` is in early development stage

## How does it work?

`bastrd` has 3 components:

1. `bastrd sync`, an agent to sync AWS IAM groups and users to Linux
1. `bastrd authorized-keys`, SSH authorized keys command to authenticate the user login against AWS IAM registered SSH Public Keys and groups
1. `bastrd toolbox`, a session wrapper for a customizable toolbox container, the user must provide an AWS IAM account MFA token for authentication and setup of the session scoped credentials.

## Toolbox features

The toolbox container has the following features:

* Validates MFA against user's AWS IAM MFA device
* Create temporary user session AWS credentials
* Mount temporary credentials  as `/home/user/.aws/` using a tmpfs mount
* Customizable session container image for advanced tools, check `Dockerfile.toolbox` for the default settings
* Session resuming, for easier recovery of connections issues
* SSH-agent forwarding (note: doesn't work on session resuming)
* Firewall rule to block containers from hijacking the AWS EC2 instance profile used by bastrd itself
* Reduced container capabilities for improved security, e.g., no socket binding

## Installing on AWS with Terraform

This repository was configured to be used as a quick way to create a `bastrd` instance on your AWS environment, fork it and customize as necessary.

1. Clone this repo
1. Configure `main.tf` with your state and `terrraform.tfvars` for your desired settings and run `terraform init`
1. Run `terraform apply` to bootstrap the CoreOS instance and setup required AWS IAM groups
1. Now wait a few minutes while your instance starts and connect to it via `ssh -A my-iam-username@$(terraform output)`

## Uninstall

1. `terraform destroy` to remove instance and related resources
