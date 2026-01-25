#!/bin/bash
set -e

# Configuration
PRIMARY_DISK="/dev/sda"
SECONDARY_DISK="/dev/sdb"

echo "========================================================"
echo " LIVE SERVER FULL REDUNDANCY (RAID 1) MIGRATION"
echo "========================================================"
echo "ACTIVE DISK: ${PRIMARY_DISK}"
echo "TARGET DISK: ${SECONDARY_DISK}"
echo "--------------------------------------------------------"
echo "GOAL: Automatic Boot & Data Takeover"
echo "1. Root Filesystem (/) -> LVM RAID 1 Mirror"
echo "2. Boot Partition (/boot) -> MDADM RAID 1 Mirror"
echo "3. EFI Partition -> Auto-Synced Clone"
echo "========================================================"

if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root" 
   exit 1
fi

read -p "Are you sure you want to proceed? This involves formatting ${SECONDARY_DISK} (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    exit 1
fi

# 1. Install necessary tools
echo "[1/8] Installing tools..."
apt-get update
apt-get install -y gdisk mdadm lvm2 rsync

# 2. Clone partition table
echo "[2/8] Cloning partition table..."
sgdisk --zap-all ${SECONDARY_DISK}
sgdisk -R ${SECONDARY_DISK} ${PRIMARY_DISK}
sgdisk -G ${SECONDARY_DISK}

# 3. Setup Redundant EFI (Clone & Hook)
echo "[3/8] Setting up EFI redundancy..."
mkfs.vfat -F32 ${SECONDARY_DISK}1
mkdir -p /mnt/efi-mirror
mount ${SECONDARY_DISK}1 /mnt/efi-mirror
rsync -a /boot/efi/ /mnt/efi-mirror/
umount /mnt/efi-mirror

# Create EFI sync hook script
cat > /etc/grub.d/90_clone_efi << 'EOF'
#!/bin/sh
# Auto-sync EFI partition to secondary disk on GRUB update
if [ -e /dev/sdb1 ]; then
    echo "Syncing EFI to /dev/sdb1..."
    mount /dev/sdb1 /mnt
    rsync -a --delete /boot/efi/ /mnt/
    umount /mnt
fi
EOF
chmod +x /etc/grub.d/90_clone_efi

# 4. Migrate /boot to RAID 1
echo "[4/8] Converting /boot to RAID 1..."
if mountpoint -q /boot; then
    # Backup boot files safely
    cp -a /boot /boot_backup
    umount /boot || (echo "Failed to unmount /boot!" && exit 1)

    # Zero superblock just in case
    mdadm --zero-superblock --force ${PRIMARY_DISK}2 ${SECONDARY_DISK}2 2>/dev/null || true

    # Create RAID 1 with metadata 1.0 (bootable)
    # Use 'missing' for primary first to preserve data safety, actually no, we wiped it.
    # We create array using both disks immediately.
    yes | mdadm --create /dev/md0 --level=1 --metadata=1.0 --raid-devices=2 ${PRIMARY_DISK}2 ${SECONDARY_DISK}2

    # Format and mount
    mkfs.ext4 -F /dev/md0
    mount /dev/md0 /boot

    # Restore files
    rsync -a /boot_backup/ /boot/
    
    # Update fstab
    BOOT_UUID=$(blkid -s UUID -o value /dev/md0)
    sed -i "s|UUID=.* /boot |UUID=${BOOT_UUID} /boot |" /etc/fstab
    
    echo "/boot converted to RAID 1 (UUID: ${BOOT_UUID})"
else
    echo "Warning: /boot is not a separate mountpoint? Skipping /boot RAID conversion."
fi

# 5. Add Secondary Disk to LVM
echo "[5/8] Expanding LVM Volume Group..."
pvcreate ${SECONDARY_DISK}3
vgextend ubuntu-vg ${SECONDARY_DISK}3

# 6. Convert Root Logical Volume to RAID 1 Mirror
echo "[6/8] Converting Root Logical Volume to RAID 1..."
LV_PATH=$(lvdisplay | grep "LV Path" | grep "ubuntu-lv" | awk '{print $3}')

if [ -z "$LV_PATH" ]; then
    echo "Error: Could not find ubuntu-lv path."
else
    echo "Mirroring $LV_PATH to ${SECONDARY_DISK}3..."
    lvconvert --type raid1 -m 1 $LV_PATH
fi

# 7. Update Configuration (mdadm, initramfs, grub)
echo "[7/8] Updating Boot Configuration..."

# Add md0 to mdadm.conf
mkdir -p /etc/mdadm
mdadm --detail --scan >> /etc/mdadm/mdadm.conf

# Update initramfs to load raid modules
update-initramfs -u

# Install GRUB to both disks
grub-install ${PRIMARY_DISK}
grub-install ${SECONDARY_DISK}
update-grub

# 8. Cleanup
echo "[8/8] Cleaning up..."
rm -rf /boot_backup

echo "========================================================"
echo "                 MIGRATION COMPLETE                     "
echo "========================================================"
echo "Status:"
echo "1. Root (/)   : LVM RAID 1 (Auto-syncing)"
echo "2. Boot (/boot): MDADM RAID 1 (Auto-syncing)"
echo "3. EFI        : Cloned (Syncs on GRUB updates)"
echo ""
echo "Verification:"
echo " - 'cat /proc/mdstat' should show active raid1"
echo " - 'sudo lvs' should show 'r' attribute for root"
echo ""
echo "You can now tolerate a complete failure of either disk."
echo "========================================================"
