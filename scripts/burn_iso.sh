#!/bin/bash

# Default to /dev/disk5 based on previous inspection
USB_DEV="/dev/disk5"
# Check for -y flag for non-interactive mode
FORCE="n"
if [ "$1" == "-y" ]; then
    FORCE="y"
    ISO_PATH="$2"
else
    ISO_PATH="$1"
fi

if [ -z "$ISO_PATH" ]; then
    echo "Usage: ./scripts/burn_iso.sh [-y] <path-to-ubuntu-live-server.iso>"
    echo "Example: ./scripts/burn_iso.sh ~/Downloads/ubuntu-22.04.3-live-server-amd64.iso"
    exit 1
fi

if [ ! -f "$ISO_PATH" ]; then
    echo "Error: ISO file not found at $ISO_PATH"
    exit 1
fi

echo "========================================================"
echo "WARNING: THIS WILL ERASE ALL DATA ON $USB_DEV (30.8 GB)"
echo "Target: Ubuntu Server RAID 1 Setup"
echo "========================================================"
diskutil list $USB_DEV
echo "========================================================"

if [ "$FORCE" == "y" ]; then
    echo "Force mode enabled. Proceeding without confirmation."
    confirm="y"
else
    read -p "Are you absolutely sure you want to write to $USB_DEV? (y/N): " confirm
fi

if [[ "$confirm" != "y" ]]; then
    echo "Aborted."
    exit 0
fi

echo "1. Unmounting disk..."
diskutil unmountDisk $USB_DEV

echo "2. Writing ISO (using /dev/rdisk5 for speed)..."
echo "   This may take a few minutes."
sudo dd if="$ISO_PATH" of=/dev/rdisk5 bs=1m status=progress

echo "3. Ejecting..."
diskutil eject $USB_DEV

echo "========================================================"
echo "Success! The USB is ready to boot."
echo "NOTE: To use the RAID 1 Autoinstall config we created,"
echo "you need to copy 'docs/SERVER_PROVISIONING_RAID1.md' (the YAML part)"
echo "to a file named 'user-data' on a SECOND USB stick labeled 'CIDATA',"
echo "OR paste it manually if you use the Autoinstall generator."
echo "========================================================"
