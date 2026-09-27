#!/bin/sh
set -eu
# Certbot deploy hook: reload only after a successful renewal of this certificate.
if [ "${RENEWED_LINEAGE:-}" = /etc/letsencrypt/live/p3express.avari.dev ]; then
    /usr/bin/openresty -p /opt/om/nginx/ -t
    /usr/bin/openresty -p /opt/om/nginx/ -s reload
fi
