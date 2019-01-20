data "aws_iam_user" "users" {
  count     = "${length(var.ssh_users)}"
  user_name = "${element(var.ssh_users, count.index)}"
}

resource "aws_iam_group" "ssh" {
  name = "${var.ssh_group_name}"
}

resource "aws_iam_group_membership" "ssh_users" {
  name  = "${aws_iam_group.ssh.name}"
  group = "${aws_iam_group.ssh.name}"
  users = ["${data.aws_iam_user.users.*.user_name}"]
}

### operator group
resource "aws_iam_group" "operators" {
  name = "${var.env}-operators"
}

resource "aws_iam_group_membership" "operators" {
  name  = "${aws_iam_group.operators.name}"
  group = "${aws_iam_group.operators.name}"
  users = ["rochacon"]
}

resource "aws_iam_group_policy" "operators" {
  name  = "${var.env}-operators"
  group = "${aws_iam_group.operators.id}"

  // FIXME add sourceIP condition
  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "201919221130",
            "Effect": "Allow",
            "Action": "sts:AssumeRole",
            "Resource": "${aws_iam_role.operator.arn}"
        },
        {
          "Sid": "201919221132",
          "Action": [
            "iam:ListAccessKeys",
            "iam:ListVirtualMFADevices"
          ],
          "Effect": "Allow",
          "Resource": "arn:aws:iam::${data.aws_caller_identity.current.account_id}:user/$${aws:username}"
        }
    ]
}
EOF
}

resource "aws_iam_role" "operator" {
  name = "${var.env}-operator"
  path = "/"

  assume_role_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Principal": {"AWS":"arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"},
        "Action": "sts:AssumeRole",
        "Condition": {"BoolIfExists": {"aws:MultiFactorAuthPresent": true}}
      }
    ]
}
EOF
}

resource "aws_iam_role_policy" "operator" {
  name = "${var.env}-operator"
  role = "${aws_iam_role.operator.id}"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "ec2:DescribeInstances"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}
