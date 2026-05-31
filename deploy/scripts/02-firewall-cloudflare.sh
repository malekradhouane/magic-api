#!/usr/bin/env bash
# ============================================================================
# MAGIC — Lock HTTP/HTTPS to Cloudflare only
# ----------------------------------------------------------------------------
# After this, ports 80/443 accept traffic ONLY from Cloudflare's network.
# This hides your origin and blocks attackers who found your raw IP from
# bypassing Cloudflare's DDoS/WAF protection.
#
# Run AFTER 01-server-setup.sh and AFTER your domain is proxied (orange cloud)
# through Cloudflare:   sudo bash 02-firewall-cloudflare.sh
#
# NOTE: only run this once Cloudflare proxying works, otherwise you lock
#       yourself out of HTTP(S). SSH (22) is unaffected.
# ============================================================================
set -euo pipefail
if [[ $EUID -ne 0 ]]; then echo "Run as root."; exit 1; fi

echo "==> Removing open 80/443 rules"
ufw delete allow 80/tcp  || true
ufw delete allow 443/tcp || true

echo "==> Allowing 80/443 from Cloudflare IPv4 ranges only"
for ip in \
  173.245.48.0/20 103.21.244.0/22 103.22.200.0/22 103.31.4.0/22 \
  141.101.64.0/18 108.162.192.0/18 190.93.240.0/20 188.114.96.0/20 \
  197.234.240.0/22 198.41.128.0/17 162.158.0.0/15 104.16.0.0/13 \
  104.24.0.0/14 172.64.0.0/13 131.0.72.0/22; do
  ufw allow from "$ip" to any port 80  proto tcp
  ufw allow from "$ip" to any port 443 proto tcp
done

echo "==> Allowing 80/443 from Cloudflare IPv6 ranges only"
for ip in \
  2400:cb00::/32 2606:4700::/32 2803:f800::/32 2405:b500::/32 \
  2405:8100::/32 2a06:98c0::/29 2c0f:f248::/32; do
  ufw allow from "$ip" to any port 80  proto tcp
  ufw allow from "$ip" to any port 443 proto tcp
done

ufw reload
echo "==> Done. 80/443 now restricted to Cloudflare. SSH unaffected."
echo "    Verify with: ufw status numbered"
