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

# example admin policy w/ MFA token requirement
resource "aws_iam_group" "admins" {
  name = "${var.env}-admins"
}

resource "aws_iam_group_membership" "admins" {
  name  = "${aws_iam_group.admins.name}"
  group = "${aws_iam_group.admins.name}"
  users = ["rochacon"]
}

resource "aws_iam_group_policy" "admins" {
  name   = "${var.env}-admins"
  group  = "${aws_iam_group.admins.id}"
  policy = "${data.aws_iam_policy_document.admins.json}"
}

data "aws_iam_policy_document" "admins" {
  statement {
    effect    = "Allow"
    actions   = ["iam:GetUser", "sts:GetSessionToken"]
    resources = ["arn:aws:iam::${data.aws_caller_identity.current.account_id}:user/$${aws:username}"]
  }

  statement {
    effect    = "Allow"
    actions   = ["*"]
    resources = ["*"]

    condition {
      test     = "Null"
      variable = "aws:MultiFactorAuthPresent"
      values   = ["false"]
    }

    condition {
      test     = "NumericLessThanEquals"
      variable = "aws:MultiFactorAuthAge"
      values   = ["3600"]
    }
  }
}
