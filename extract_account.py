import json

try:
    with open('zone_info.json', 'r') as f:
        data = json.load(f)
    print(data['result'][0]['account']['id'])
except Exception as e:
    print("Error:", e)
