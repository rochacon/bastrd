data "ignition_config" "userdata" {
  files = [
    data.ignition_file.bastrd.rendered,
    data.ignition_file.bastrd_toolbox.rendered,
    data.ignition_file.pam_sshd.rendered,
    data.ignition_file.pam_sudo.rendered,
    data.ignition_file.sudoers.rendered,
    data.ignition_file.sshd_config.rendered,
  ]

  systemd = [
    data.ignition_systemd_unit.update-engine.rendered,
    data.ignition_systemd_unit.locksmithd.rendered,
    data.ignition_systemd_unit.docker_block_ec2_metadata.rendered,
    data.ignition_systemd_unit.bastrd_sync.rendered,
  ]
}

// Disable update-engine and locksmithd to avoid undesirable instance
// update reboots
data "ignition_systemd_unit" "update-engine" {
  name = "update-engine.service"
  mask = true
}

data "ignition_systemd_unit" "locksmithd" {
  name = "locksmithd.service"
  mask = true
}

// Block AWS EC2 metadata access from the containers to avoid bastrd
// IAM instance profile hijacking
data "ignition_systemd_unit" "docker_block_ec2_metadata" {
  name = "docker.service"

  dropin {
    name = "10-block-ec2-metadata.conf"

    content = <<EOF
[Service]
ExecStartPost=/usr/sbin/iptables -I DOCKER-USER -i docker0 -d 169.254.169.254/32 -j REJECT
EOF

  }
}

// SSHd config for AuthorizedKeysCommand
data "ignition_file" "sshd_config" {
  filesystem = "root"
  path       = "/etc/ssh/sshd_config"
  mode       = 384

  content {
    content = <<EOF
AllowAgentForwarding yes
AllowGroups ${var.ssh_group_name}
AllowStreamLocalForwarding no
AllowTcpForwarding no
AuthenticationMethods publickey,keyboard-interactive:pam
AuthorizedKeysCommand /opt/bin/bastrd authorized-keys --allowed-group=${var.ssh_group_name} %u
AuthorizedKeysCommandUser nobody
ChallengeResponseAuthentication yes
ClientAliveInterval 30
MaxAuthTries 3
PermitEmptyPasswords no
PermitRootLogin no
PrintLastLog yes
PrintMotd no
UseDNS no
UsePAM yes
EOF

  }
}

// Install bastrd to /opt/bin
data "ignition_file" "bastrd" {
  filesystem = "root"
  path       = "/opt/bin/bastrd"
  mode       = 493

  source {
    // FIXME add sha256 integrity check and download from GitHub release
    source      = "https://s3.amazonaws.com/bastrd-dev/bastrd.gz"
    compression = "gzip"
  }
}

// bastrd toolbox wrapper script for call from SSH session
data "ignition_file" "bastrd_toolbox" {
  filesystem = "root"
  path       = "/opt/bin/bastrd-toolbox"
  mode       = 493

  content {
    content = <<EOF
#!/bin/bash
export AWS_DEFAULT_REGION="${var.region}"
/opt/bin/bastrd toolbox --image=${var.toolbox_image} --username=$${USER} "$${@}"
EOF

  }
}

// bastrd sync service to synchronize AWS IAM users every minute
data "ignition_systemd_unit" "bastrd_sync" {
  name = "bastrd-sync.service"

  content = <<EOF
[Unit]
Description=bastrd sync AWS IAM groups and users
After=syslog.target network.target auditd.service

[Service]
Restart=always
RestartSec=10
Environment=AWS_DEFAULT_REGION=${var.region}
ExecStart=/opt/bin/bastrd sync --interval=1m --group=${var.ssh_group_name}

[Install]
WantedBy=multi-user.target
EOF

}

// bastrd integration with pam for password check against AWS IAM
data "ignition_file" "pam_sshd" {
  filesystem = "root"
  path       = "/etc/pam.d/sshd"
  mode       = 384

  content {
    content = <<EOF
auth  sufficient                  pam_exec.so expose_authtok quiet stdout /opt/bin/bastrd pam
auth  [success=1 default=ignore]  pam_unix.so nullok_secure
auth  requisite                   pam_deny.so
auth  required                    pam_permit.so

account   required    pam_unix.so
account   optional    pam_permit.so

session   required    pam_limits.so
session   required    pam_env.so
session   required    pam_unix.so
session   optional    pam_permit.so
-session  optional    pam_systemd.so
EOF

  }
}

data "ignition_file" "pam_sudo" {
  filesystem = "root"
  path       = "/etc/pam.d/sudo"
  mode       = 384

  content {
    content = <<EOF
auth  sufficient                  pam_exec.so expose_authtok quiet stdout /opt/bin/bastrd pam --skip-credential-update
auth  [success=1 default=ignore]  pam_unix.so nullok_secure
auth  requisite                   pam_deny.so
auth  required                    pam_permit.so

account   required    pam_unix.so
account   optional    pam_permit.so

session   required    pam_limits.so
session   required    pam_env.so
session   required    pam_unix.so
session   optional    pam_permit.so
-session  optional    pam_systemd.so
EOF

  }
}

data "ignition_file" "sudoers" {
  filesystem = "root"
  path       = "/etc/sudoers.d/default"
  mode       = 384

  content {
    content = <<EOF
## Based on https://github.com/coreos/baselayout/blob/master/baselayout/sudoers
## Pass LESSCHARSET through for systemd commands run through sudo that call less.
## See https://github.com/coreos/bugs/issues/365.
Defaults env_keep += "LESSCHARSET"

## enable root and ${var.ssh_group_name} group
root ALL=(ALL) ALL
%${var.ssh_group_name} ALL=(ALL) ALL
EOF

  }
}
