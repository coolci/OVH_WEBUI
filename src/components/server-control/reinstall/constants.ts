import { detectOsKind } from "@/components/server-control/OsIcon";

/** OS 分组(标签走 i18n key,渲染处 t())+ 显示顺序(按用户使用频率排) */
export const OS_GROUPS: { kind: ReturnType<typeof detectOsKind>; label: string }[] = [
  { kind: "debian",   label: "ctrl.reinstall.os.debian" },
  { kind: "ubuntu",   label: "ctrl.reinstall.os.ubuntu" },
  { kind: "windows",  label: "ctrl.reinstall.os.windows" },
  { kind: "proxmox",  label: "ctrl.reinstall.os.proxmox" },
  { kind: "rocky",    label: "ctrl.reinstall.os.rocky" },
  { kind: "alma",     label: "ctrl.reinstall.os.alma" },
  { kind: "fedora",   label: "ctrl.reinstall.os.fedora" },
  { kind: "esxi",     label: "ctrl.reinstall.os.esxi" },
  { kind: "centos",   label: "ctrl.reinstall.os.centos" },
  { kind: "opensuse", label: "ctrl.reinstall.os.opensuse" },
  { kind: "freebsd",  label: "ctrl.reinstall.os.freebsd" },
  { kind: "byoi",     label: "ctrl.reinstall.os.byoi" },
  { kind: "byolinux", label: "ctrl.reinstall.os.byolinux" },
  { kind: "linux",    label: "ctrl.reinstall.os.linux" },
];

export const HARDWARE_RAID_LEVELS = [
  { value: "", label: "ctrl.reinstall.hwRaid.none" },
  { value: "raid0", label: "ctrl.reinstall.hwRaid.raid0" },
  { value: "raid1", label: "ctrl.reinstall.hwRaid.raid1" },
  { value: "raid5", label: "ctrl.reinstall.hwRaid.raid5" },
  { value: "raid6", label: "ctrl.reinstall.hwRaid.raid6" },
  { value: "raid10", label: "ctrl.reinstall.hwRaid.raid10" },
];

export const SOFTWARE_RAID_LEVELS = [
  { value: "raid0", label: "ctrl.reinstall.swRaid.raid0" },
  { value: "raid1", label: "ctrl.reinstall.swRaid.raid1" },
  { value: "raid5", label: "ctrl.reinstall.swRaid.raid5" },
  { value: "raid6", label: "ctrl.reinstall.swRaid.raid6" },
  { value: "raid7", label: "ctrl.reinstall.swRaid.raid7" },
  { value: "raid10", label: "ctrl.reinstall.swRaid.raid10" },
];

export const FILESYSTEMS = [
  "ext4",
  "ext3",
  "xfs",
  "btrfs",
  "zfs",
  "swap",
  "reiserfs",
  "ntfs",
  "fat16",
  "ufs",
  "vmfs5",
  "vmfs6",
  "vmfsl",
  "none",
];

export type StorageMode = "default" | "zfs" | "scheme" | "custom";
