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
