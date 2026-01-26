import json

with open('manual_dns_check.json', 'r') as f:
    data = json.load(f)

for record in data['result']:
    if record['type'] == 'CNAME' and 'cfargotunnel.com' in record['content']:
        print(record['id'])
