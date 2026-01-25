import os
import requests
import sys
import time
import urllib3

# Configuration
BASE_URL = "https://app.gurukripacoldstore.in"
LOGIN_URL = f"{BASE_URL}/auth/login"
UPLOAD_URL = f"{BASE_URL}/api/files/upload"
USERNAME = "user@cold.in"
PASSWORD = "1" * 8
SOURCE_DIR = os.path.expanduser("~/Downloads/120 Bahadur (2025) [1080p] [WEBRip] [x265] [10bit] [5.1] [YTS.LT]")
ROOT_BUCKET = "bulk" 

# Disable SSL warnings for local/dev certs
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

def login():
    print(f"🔑 Authenticating as {USERNAME}...")
    try:
        resp = requests.post(LOGIN_URL, json={"email": USERNAME, "password": PASSWORD}, verify=False)
        if resp.status_code == 200:
            token = resp.json().get("token")
            print("✅ Login successful!")
            return token
        else:
            print(f"❌ Login failed: {resp.text}")
            sys.exit(1)
    except Exception as e:
        print(f"❌ Connection error: {e}")
        sys.exit(1)

def upload_folder(token):
    if not os.path.exists(SOURCE_DIR):
        print(f"❌ Directory not found: {SOURCE_DIR}")
        sys.exit(1)

    print(f"📂 Scanning directory: {SOURCE_DIR}")
    files_to_upload = []
    
    # Walk directory to find all files
    for root, dirs, files in os.walk(SOURCE_DIR):
        for file in files:
            full_path = os.path.join(root, file)
            # Calculate relative path from the *parent* of SOURCE_DIR so the main folder name is included
            rel_path = os.path.relpath(full_path, os.path.dirname(SOURCE_DIR))
            dest_dir = os.path.dirname(rel_path)
            
            files_to_upload.append({
                "full_path": full_path,
                "dest_path": dest_dir,
                "filename": file
            })
            
    total_files = len(files_to_upload)
    print(f"🚀 Found {total_files} files to upload.")
    
    headers = {"Authorization": f"Bearer {token}"}
    
    for i, item in enumerate(files_to_upload, 1):
        fpath = item["full_path"]
        dpath = item["dest_path"]
        fname = item["filename"]
        
        file_size = os.path.getsize(fpath)
        size_str = f"{file_size / (1024*1024):.2f} MB"
        
        print(f"\n[{i}/{total_files}] Uploading: {fname} ({size_str})")
        print(f"    Target: {dpath}")
        
        try:
            with open(fpath, 'rb') as f:
                files = {'file': (fname, f)}
                data = {'root': ROOT_BUCKET, 'path': dpath}
                
                start_time = time.time()
                resp = requests.post(UPLOAD_URL, headers=headers, files=files, data=data, verify=False)
                elapsed = time.time() - start_time
                
                if resp.status_code == 200:
                    speed = (file_size / (1024*1024)) / elapsed if elapsed > 0 else 0
                    print(f"    ✅ Success ({elapsed:.1f}s @ {speed:.2f} MB/s)")
                else:
                    print(f"    ❌ Failed: {resp.status_code} - {resp.text}")
        except Exception as e:
            print(f"    ❌ Error: {e}")

if __name__ == "__main__":
    token = login()
    upload_folder(token)
