# Post-Installation RAID 1 Conversion Guide

Since you installed Ubuntu on a single disk (e.g., `/dev/sda`), this guide explains how to migrate to a **RAID 1 Mirror** with the second disk (`/dev/sdb`) without reinstalling.

**WARNING:** This procedure involves modifying bootloaders and filesystems. **Backup important data first.**

## Phase 1: Prepare the Second Disk
Values: `Active Disk = /dev/sda` | `New Disk = /dev/sdb`

1. **Copy Partition Table**
   Copy the layout from the active disk to the new empty disk.
   ```bash
   sudo sgdisk --replicate=/dev/sdb /dev/sda
   sudo sgdisk -G /dev/sdb  # Randomize GUIDs
   ```

2. **Add New Disk to MDADM (as a "missing" array)**
   We create a RAID array with one disk missing (the one we are currently running on).
   ```bash
   # Check partition names (e.g., sdb2 is the main root partition)
   lsblk
   
   # Create RAID device /dev/md0 using sdb2 (assuming sda2 is your current root)
   # We use 'missing' for the first slot to add sda2 later.
   sudo mdadm --create /dev/md0 --level=1 --raid-devices=2 missing /dev/sdb2
   ```

3. **Format and Mount the RAID**
   ```bash
   sudo mkfs.ext4 /dev/md0
   sudo mount /dev/md0 /mnt
   ```

## Phase 2: Clone Data
Sync your running system to the new RAID array.

```bash
sudo rsync -aAXv --exclude={"/dev/*","/proc/*","/sys/*","/tmp/*","/run/*","/mnt/*","/media/*","/lost+found"} / /mnt/
```

## Phase 3: Configure Bootloader and Fstab

1. **Update Fstab on the RAID**
   Get the UUID of the new RAID device (`/dev/md0`):
   ```bash
   sudo blkid /dev/md0
   ```
   Edit `/mnt/etc/fstab`: change the UUID of `/` (root) to the UUID of `/dev/md0`.

2. **Chroot into the RAID system**
   ```bash
   sudo mount --bind /dev /mnt/dev
   sudo mount --bind /proc /mnt/proc
   sudo mount --bind /sys /mnt/sys
   sudo chroot /mnt
   ```

3. **Update GRUB (Inside Chroot)**
   ```bash
   # Install GRUB to both disks
   grub-install /dev/sda
   grub-install /dev/sdb
   
   # Update initramfs (crucial for loading raid modules)
   update-initramfs -u
   update-grub
   exit
   ```

## Phase 4: Reboot and Sync

1. **Reboot**
   Restart your server. In the BIOS boot menu, try booting from the **Second Disk** (the one with the RAID).
   

2. **Add Old Disk to Array**

   If you successfully booted into the RAID (check `df -h /` -> should be `/dev/md0`), you can now add the old disk.
   
   ```bash
   # Change partition type of old root to Linux RAID
   sudo sgdisk -t 2:fd00 /dev/sda
   
   # Add to array
   sudo mdadm --manage /dev/md0 --add /dev/sda2
   ```

3. **Wait for Sync**

   The drives will now synchronize. Check progress:

   ```bash
   watch cat /proc/mdstat
   ```

## Troubleshooting

If `grub-install` fails or the system doesn't boot, you may need to check if you are using UEFI or Legacy BIOS and ensure the EFI partitions are synced.
