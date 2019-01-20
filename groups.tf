data "aws_iam_user" "users" {
  count     = "${length(var.ssh_users)}"
  user_name = "${element(var.ssh_users, count.index)}"
}

resource "aws_iam_group" "ssh" {
  name = "${var.ssh_group_name}"
}

resource "aws_iam_group_membership" "ssh_users" {
  name  = "${aws_iam_group.ssh.name}-users"
  group = "${aws_iam_group.ssh.name}"
  users = ["${data.aws_iam_user.users.*.user_name}"]
}

resource "aws_iam_role" "users" {
  name = "${var.env}-users"
  path = "/"

  assume_role_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "201812221130",
            "Effect": "Allow",
            "Action": "sts:AssumeRole",
            "Principal": {"AWS":"${data.aws_caller_identity.current.account_id}"},
            "Condition": {
                "BoolIfExists": {
                    "aws:MultiFactorAuthPresent": "false"
                }
            }
        }
    ]
}
EOF
}
