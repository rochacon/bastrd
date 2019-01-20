data "ignition_config" "userdata" {
  files = [
    "${data.ignition_file.bastrd.id}",
    "${data.ignition_file.bastrd_toolbox.id}",
    // "${data.ignition_file.pam_sshd.id}",
    "${data.ignition_file.sshd_config.id}",
  ]

  systemd = [
    "${data.ignition_systemd_unit.update-engine.id}",
    "${data.ignition_systemd_unit.locksmithd.id}",
    "${data.ignition_systemd_unit.docker_block_ec2_metadata.id}",
    "${data.ignition_systemd_unit.bastrd_sync.id}",
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

  dropin = {
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
  mode       = 0600

  content {
    content = <<EOF
AllowAgentForwarding yes
AllowGroups ${var.ssh_group_name}
AllowStreamLocalForwarding no
AllowTcpForwarding no
AuthenticationMethods publickey
AuthorizedKeysCommand /opt/bin/bastrd authorized-keys --allowed-groups=${var.ssh_group_name} %u
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
  mode       = 0755

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
  mode       = 0755

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
ExecStart=/opt/bin/bastrd sync --disable-sandbox --interval=1m --groups=${var.ssh_group_name}

[Install]
WantedBy=multi-user.target
EOF
}

// bastrd integration with pam for on-demand user creation
data "ignition_file" "pam_sshd" {
  filesystem = "root"
  path       = "/etc/pam.d/sshd"
  mode       = 0600

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
