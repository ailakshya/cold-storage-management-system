import json

with open('debug_dns.json', 'r') as f:
    data = json.load(f)

for record in data['result']:
    print(f"{record['name']} -> {record['type']}")
