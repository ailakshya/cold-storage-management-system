#!/bin/bash

# Configuration
FILENAME="user-data"
OUTPUT_DIR="config-drive"

# Ensure output dir exists
mkdir -p "$OUTPUT_DIR"

# Create empty meta-data file (required for cloud-init)
touch "$OUTPUT_DIR/meta-data"

# Extract user-data content from the docs
# (We are taking the YAML block from SERVER_PROVISIONING_RAID1.md)

cat > "$OUTPUT_DIR/$FILENAME" <<EOF
#cloud-config
autoinstall:
  version: 1
  identity:
    hostname: cold-storage-server
    username: admin
    password: "\$6\$9y8PphWKqaODymVq\$kMBkaCyXSWLvtF81KDMcbcTfSUTeLiJa.ysOloTCbdcBBoazMVi9ORqJQXI0DDt4pSv4OUqwjfZdry6A9Tu4T1" # Password: "Lak992723/"
  ssh:
    install-server: true
    allow-pw: true
  storage:
    config:
      # Disk 1
      - type: disk
        id: disk-primary
        match:
          path: /dev/sda
      
      # Disk 2
      - type: disk
        id: disk-secondary
        match:
          path: /dev/sdb

      # Partition Disk 1 (Boot + Root)
      - type: partition
        id: boot-part-a
        device: disk-primary
        size: 1G
        flag: boot
        grub_device: true
      - type: partition
        id: root-part-a
        device: disk-primary
        size: -1

      # Partition Disk 2 (Boot + Root)
      - type: partition
        id: boot-part-b
        device: disk-secondary
        size: 1G
        flag: boot
        grub_device: true
      - type: partition
        id: root-part-b
        device: disk-secondary
        size: -1

      # RAID 1
      - type: raid
        id: md0
        name: md0
        raidlevel: 1
        devices:
          - root-part-a
          - root-part-b
      
      # Format
      - type: format
        id: format-root
        fstype: ext4
        volume: md0

      # Mount
      - type: mount
        id: mount-root
        device: format-root
        path: /
EOF

echo "========================================================"
echo "Generated Autoinstall Config at: $OUTPUT_DIR/$FILENAME"
echo "========================================================"
echo "INSTRUCTIONS FOR METHOD 1 (Automated Install):"
echo "1. Get a SECOND USB stick (any small size is fine)."
echo "2. Format it as FAT32 and name the volume 'CIDATA' (all caps)."
echo "3. Copy the files '$OUTPUT_DIR/user-data' and '$OUTPUT_DIR/meta-data'"
echo "   to the root of that USB stick."
echo "4. Plug BOTH USB sticks into the server:"
echo "   - USB 1: The Ubuntu Server Installer (Wait for download to finish first!)"
echo "   - USB 2: This CIDATA stick"
echo "5. Boot from USB 1. It will automatically find USB 2 and use the config."
echo "========================================================"
