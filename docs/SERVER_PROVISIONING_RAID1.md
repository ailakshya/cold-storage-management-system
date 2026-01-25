# Server OS Provisioning with RAID 1 (Mirror)

This guide documents the procedure for setting up a new server node for the Cold Storage Management System with a **RAID 1 (Mirror)** disk layout. This ensures high availability and data redundancy at the OS level.

## Prerequisites

- **2x SSD/NVMe Drives** of identical size (e.g., 2x 500GB or 2x 1TB).
- **Ubuntu Server 22.04 / 24.04 LTS** ISO.
- A USB Flash Drive.

---

## Method 1: Automated Installation (Recommended)

This method involves creating a custom "user-data" configuration file on your USB drive. The Ubuntu installer will read this and automatically configure the RAID layout.

### 1. Prepare the USB
1. Flash the Ubuntu Server ISO to your USB drive (using BalenaEtcher, Rufus, or `dd`).
2. Mount the "CIDATA" partition of the USB drive (or create a `user-data` file in the root if using modern Ubuntu Autoinstall methods).
3. Create/Edit the `user-data` file with the configuration below.

### 2. `user-data` Configuration
Use this YAML configuration to define a RAID 1 mirror. 

**CRITICAL WARNING:** This config assumes your target install disks are `/dev/sda` and `/dev/sdb`. 
- **Check your hardware first:** On many servers, disks are `/dev/nvme0n1` and `/dev/nvme1n1`.
- **Watch out for the USB:** Sometimes the USB stick itself loads as `/dev/sda`. If so, your target disks might be `/dev/sdb` and `/dev/sdc`. Adjust the `path:` keys below accordingly.

```yaml
#cloud-config
autoinstall:
  version: 1
  identity:
    hostname: cold-storage-server
    username: admin
    password: "$6$9y8PphWKqaODymVq$kMBkaCyXSWLvtF81KDMcbcTfSUTeLiJa.ysOloTCbdcBBoazMVi9ORqJQXI0DDt4pSv4OUqwjfZdry6A9Tu4T1" # Password: "Lak992723/"
  ssh:
    install-server: true
    allow-pw: true
  storage:
    # We use a custom config, so do NOT specify 'layout: name: direct' here.
    config:
      # Disk 1
      - type: disk
        id: disk-primary
        match:
          path: /dev/sda  # <--- VERIFY THIS PATH
      
      # Disk 2
      - type: disk
        id: disk-secondary
        match:
          path: /dev/sdb  # <--- VERIFY THIS PATH

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
        size: -1 # Rest of disk

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
        size: -1 # Rest of disk

      # Create RAID 1 Array (md0) from the two root partitions
      - type: raid
        id: md0
        name: md0
        raidlevel: 1
        devices:
          - root-part-a
          - root-part-b
      
      # Format as Ext4
      - type: format
        id: format-root
        fstype: ext4
        volume: md0

      # Mount at /
      - type: mount
        id: mount-root
        device: format-root
        path: /
```

---

## Method 2: Manual Setup via SSH (Live Installer)

If you have booted the Ubuntu Server installer and selected "Help -> Enter Shell" or enabled SSH access during the welcome screen.

**SSH Access:**
- Get the IP from the console.
- Connect: `ssh installer@<IP>` (Password is usually shown on screen).

### 1. Identify Disks
```bash
lsblk -d -o NAME,SIZE,MODEL
# Assume /dev/sda and /dev/sdb are your target disks
```

### 2. Wipe Disks (Destructive!)
```bash
# Clear partition tables
sudo sgdisk --zap-all /dev/sda
sudo sgdisk --zap-all /dev/sdb
```

### 3. Create Partitions
We need:
1. **EFI Partition** (512MB) - Cloned on both disks.
2. **RAID Partition** (Remaining space).

```bash
# Disk 1 (sda)
sudo sgdisk -n 1:0:+512M -t 1:ef00 /dev/sda # EFI
sudo sgdisk -n 2:0:0     -t 2:fd00 /dev/sda # RAID

# Disk 2 (sdb)
sudo sgdisk -n 1:0:+512M -t 1:ef00 /dev/sdb # EFI
sudo sgdisk -n 2:0:0     -t 2:fd00 /dev/sdb # RAID
```

### 4. Create RAID 1 Array
```bash
# Create /dev/md0 from partition 2 of both disks
sudo mdadm --create /dev/md0 --level=1 --raid-devices=2 /dev/sda2 /dev/sdb2
```

### 5. Format and LVM (Optional but recommended)
```bash
# Initialize Physical Volume
sudo pvcreate /dev/md0

# Create Volume Group
sudo vgcreate vg_system /dev/md0

# Create Logical Volume for Root
sudo lvcreate -l 100%FREE -n lv_root vg_system

# Format Root
sudo mkfs.ext4 /dev/vg_system/lv_root

# Format EFI (both disks for redundancy)
sudo mkfs.vfat /dev/sda1
sudo mkfs.vfat /dev/sdb1
```

### 6. Mount and Proceed

Instructions for "curtin" installation (advanced) would follow here.

**However**, if you are in the Ubuntu Installer UI:
1. Select "Custom Storage Layout".
2. Mark `sda2` and `sdb2` as "Leave unformatted".
3. Create a Software RAID (MD) device using `sda2` and `sdb2`.
4. Format the resulting MD device as Ext4 and mount at `/`.

---

## Recovery / Verification

To check the status of your RAID array after installation:

```bash
cat /proc/mdstat
# Should show [UU] (both drives UP)

sudo mdadm --detail /dev/md0
```
