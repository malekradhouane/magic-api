#!/usr/bin/env bash
# ============================================================================
# MAGIC — Initial server hardening (Ubuntu 22.04/24.04 on Hetzner)
# ----------------------------------------------------------------------------
# Run as root on a FRESH VPS:  sudo bash 01-server-setup.sh
#
# What it does:
#   - Creates a non-root sudo user with your SSH key
#   - Disables password & root SSH login (key-only)
#   - Installs & configures UFW firewall (only 22/80/443)
#   - Installs fail2ban (brute-force protection on SSH)
#   - Enables automatic security updates
#   - Installs Docker + Docker Compose plugin
#
# EDIT the variables below BEFORE running.
# ============================================================================
set -euo pipefail

# ─── EDIT ME ────────────────────────────────────────────────────────────────
NEW_USER="magic"
SSH_PUBKEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILMyOwqVV+34PSqttzNL/Gt/belHxQ67n+4goGS6MxOE malek.radhouen@ifollow.fr"
SSH_PORT="22"   # keep 22, or change (then update UFW + your ssh config)
# ─────────────────────────────────────────────────────────────────────────────

if [[ $EUID -ne 0 ]]; then echo "Run as root."; exit 1; fi
if [[ "$SSH_PUBKEY" == ssh-ed25519\ AAAA...* ]]; then
  echo "ERROR: set SSH_PUBKEY to your real public key first."; exit 1
fi

echo "==> System update"
apt-get update -y && apt-get upgrade -y

echo "==> Create user '$NEW_USER'"
if ! id "$NEW_USER" &>/dev/null; then
  adduser --disabled-password --gecos "" "$NEW_USER"
fi
usermod -aG sudo "$NEW_USER"
install -d -m 700 -o "$NEW_USER" -g "$NEW_USER" "/home/$NEW_USER/.ssh"
echo "$SSH_PUBKEY" > "/home/$NEW_USER/.ssh/authorized_keys"
chmod 600 "/home/$NEW_USER/.ssh/authorized_keys"
chown "$NEW_USER:$NEW_USER" "/home/$NEW_USER/.ssh/authorized_keys"

echo "==> Harden SSH (key-only, no root)"
sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/'   /etc/ssh/sshd_config
sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin no/'                 /etc/ssh/sshd_config
sed -i 's/^#\?PubkeyAuthentication.*/PubkeyAuthentication yes/'      /etc/ssh/sshd_config
sed -i "s/^#\?Port .*/Port ${SSH_PORT}/"                            /etc/ssh/sshd_config
systemctl restart ssh || systemctl restart sshd

echo "==> Firewall (UFW)"
apt-get install -y ufw
ufw default deny incoming
ufw default allow outgoing
ufw allow "${SSH_PORT}/tcp"
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

echo "==> fail2ban"
apt-get install -y fail2ban
cat >/etc/fail2ban/jail.local <<EOF
[sshd]
enabled  = true
port     = ${SSH_PORT}
maxretry = 4
bantime  = 1h
findtime = 10m
EOF
systemctl enable --now fail2ban
systemctl restart fail2ban

echo "==> Automatic security updates"
apt-get install -y unattended-upgrades
dpkg-reconfigure -f noninteractive unattended-upgrades

echo "==> Docker"
if ! command -v docker &>/dev/null; then
  curl -fsSL https://get.docker.com | sh
fi
usermod -aG docker "$NEW_USER"

echo
echo "============================================================"
echo " DONE. Now:"
echo "  1) Open a NEW terminal and test:  ssh -p ${SSH_PORT} ${NEW_USER}@<server-ip>"
echo "  2) Do NOT close this session until the new login works."
echo "  3) Next: run 02-firewall-cloudflare.sh to lock 443 to Cloudflare."
echo "============================================================"
