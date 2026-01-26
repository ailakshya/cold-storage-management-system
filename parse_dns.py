import json
import logging

try:
    with open('dns_records.json', 'r') as f:
        data = json.load(f)

    if not data.get('success'):
        print(f"Error fetching records: {data}")
        exit(1)

    records = data.get('result', [])
    to_delete = []

    targets = ['app.gurukripacoldstore.in', 'customer.gurukripacoldstore.in', 'gurukripacoldstore.in', 'www.gurukripacoldstore.in']

    print(f"Found {len(records)} records.")

    for record in records:
        name = record['name']
        type = record['type']
        id = record['id']
        
        print(f"Record: {name} ({type}) - {id}")

        if name in targets and type in ['A', 'CNAME', 'AAAA']:
            to_delete.append(id)
            print(f"  -> MARKED FOR DELETION")

    with open('to_delete.txt', 'w') as f:
        for id in to_delete:
            f.write(f"{id}\n")

except Exception as e:
    print(f"Failed: {e}")
