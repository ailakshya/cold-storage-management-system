#!/bin/bash
ACCOUNT_ID="8ac6054e727fbfd99ced86c9705a5893"
TUNNEL_ID="e455d8e4-a524-4393-8bab-9df1372107a3"

curl -X PUT "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/tunnels/$TUNNEL_ID/configurations" \
     -H "Authorization: Bearer CW4zp6SaQjuD55Ugqgq7xG5Qw-5aWFtAVzTM-mSD" \
     -H "Content-Type: application/json" \
     --data '{
       "config": {
         "ingress": [
           {
             "hostname": "gurukripacoldstore.in",
             "service": "http://localhost:80"
           },
           {
             "hostname": "www.gurukripacoldstore.in",
             "service": "http://localhost:80"
           },
           {
             "hostname": "app.gurukripacoldstore.in",
             "service": "http://localhost:80"
           },
           {
             "hostname": "customer.gurukripacoldstore.in",
             "service": "http://localhost:80"
           },
           {
             "service": "http_status:404"
           }
         ]
       }
     }'

