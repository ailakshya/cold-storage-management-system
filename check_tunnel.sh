#!/bin/bash
ACCOUNT_ID="8ac6054e727fbfd99ced86c9705a5893"
TUNNEL_ID="e455d8e4-a524-4393-8bab-9df1372107a3"

# Fetch current config to check if we can append or need to replace
echo "Fetching current tunnel configuration..."
curl -s -X GET "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/tunnels/$TUNNEL_ID/configurations" \
     -H "Authorization: Bearer CW4zp6SaQjuD55Ugqgq7xG5Qw-5aWFtAVzTM-mSD" \
     -H "Content-Type: application/json"

