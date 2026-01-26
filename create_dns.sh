#!/bin/bash
ZONE_ID="053bd8b979c07c065a2a62cc051f930f"
TUNNEL_DOMAIN="e455d8e4-a524-4393-8bab-9df1372107a3.cfargotunnel.com"

# 1. gurukripacoldstore.in (root)
echo "Creating root record..."
curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records" \
     -H "Authorization: Bearer CW4zp6SaQjuD55Ugqgq7xG5Qw-5aWFtAVzTM-mSD" \
     -H "Content-Type: application/json" \
     --data "{\"type\":\"CNAME\",\"name\":\"@\",\"content\":\"$TUNNEL_DOMAIN\",\"proxied\":true}"

# 2. www.gurukripacoldstore.in
echo "Creating www record..."
curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records" \
     -H "Authorization: Bearer CW4zp6SaQjuD55Ugqgq7xG5Qw-5aWFtAVzTM-mSD" \
     -H "Content-Type: application/json" \
     --data "{\"type\":\"CNAME\",\"name\":\"www\",\"content\":\"$TUNNEL_DOMAIN\",\"proxied\":true}"

# 3. app.gurukripacoldstore.in
echo "Creating app record..."
curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records" \
     -H "Authorization: Bearer CW4zp6SaQjuD55Ugqgq7xG5Qw-5aWFtAVzTM-mSD" \
     -H "Content-Type: application/json" \
     --data "{\"type\":\"CNAME\",\"name\":\"app\",\"content\":\"$TUNNEL_DOMAIN\",\"proxied\":true}"

# 4. customer.gurukripacoldstore.in
echo "Creating customer record..."
curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records" \
     -H "Authorization: Bearer CW4zp6SaQjuD55Ugqgq7xG5Qw-5aWFtAVzTM-mSD" \
     -H "Content-Type: application/json" \
     --data "{\"type\":\"CNAME\",\"name\":\"customer\",\"content\":\"$TUNNEL_DOMAIN\",\"proxied\":true}"

