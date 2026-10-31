export interface OwnedServer {
  serviceName: string;
  name: string;
  commercialRange: string;
  datacenter: string;
  state: string;
  status?: string;
  ip: string;
  os: string;
  orderId?: string | number;
  reverse?: string;
  monitoring?: boolean;
  professionalUse?: boolean;
  bootId?: number | null;
  renewalType?: boolean | null;
  svcInfoError?: string;
  error?: string;
  /** 中奖摘要(后端 1h 缓存,只有确实中奖的机器才带)。徽章用,详情在概览页 */
  lottery?: { won: boolean; tier?: number };
}

export interface HardwareLotteryItem {
  kind: "cpu" | "memory" | "disk";
  ordered: string;
  actual: string;
  /** 量化涨幅(%),0/缺省 = 算不出来 */
  gainPct?: number;
  /** 硬盘介质升级(HDD→SSD→NVMe) */
  mediaUp?: boolean;
}

export interface HardwareLottery {
  checked: boolean;
  won: boolean;
  /** 1=中奖 2=大奖 3=头奖(中奖项数 + 翻倍加成,后端算好) */
  tier?: number;
  planCode?: string;
  planName?: string;
  items: HardwareLotteryItem[];
  reason?: string;
}

export interface HardwareInfo {
  processorName: string;
  processorArchitecture: string;
  coresPerProcessor: number;
  threadsPerProcessor: number;
  memorySize?: { value: number; unit: string };
  diskGroups?: any[];
  expansionCards?: any[];
  lottery?: HardwareLottery | null;
}

export interface ServiceInfo {
  status: string;
  expiration: string;
  creation: string;
  renewalType: boolean;
  renewalPeriod: number;
  renewalDeleteAtExpiration: boolean;
  terminationScheduled?: boolean;
  terminationAction?: string;
  terminationStateUnknown?: boolean;
  terminationDate?: string;
  renewalForced: boolean;
  renewalManualPayment: boolean;
  /** 服务级续费能力(automatic / manual);缺省 = 没读到,由 OVH 在提交时校验 */
  renewalModes?: string[] | null;
  possibleRenewPeriod: number[];
}

export interface EngagementPricing {
  pricingMode: string;
  description: string;
  duration: string;
  interval: number;
  price: { value: number; currencyCode?: string; text?: string };
  priceInUcents?: number;
  engagementConfiguration?: {
    defaultEndAction: string;
    duration: string;
    type: string;
  };
}

export interface EngagementInfo {
  currentPeriod?: { startDate: string; endDate: string };
  endRule?: { strategy: string; possibleStrategies: string[] };
}

export interface EngagementRequest {
  pricing?: EngagementPricing;
  requestDate?: string;
  order?: { orderId?: number; url?: string };
}

export type EngagementEndStrategy =
  | "CANCEL_SERVICE"
  | "REACTIVATE_ENGAGEMENT"
  | "STOP_ENGAGEMENT_FALLBACK_DEFAULT_PRICE"
  | "STOP_ENGAGEMENT_KEEP_PRICE";

export interface MitigationIp {
  ipOnMitigation: string;
  state: string;
  auto: boolean;
  permanent: boolean;
  error?: string;
  errorCode?: string;
  detail?: string;
  queryId?: string;
}

export interface MitigationBlock {
  ipBlock: string;
  mitigations: MitigationIp[];
  error?: string;
  errorCode?: string;
  detail?: string;
  queryId?: string;
}

export interface DetailErrorMarked {
  _detailError?: string;
}

export interface NetworkInterface extends DetailErrorMarked {
  mac: string;
  linkType?: string;
}

export interface BootMode {
  id: number;
  bootType: string;
  description: string;
  kernel: string;
  active: boolean;
  error?: string;
}

export interface ServerTask {
  taskId: number;
  function: string;
  status: string;
  startDate: string;
  doneDate: string;
  comment?: string;
  error?: string;
}

export interface CustomizeQuestion {
  name: string;
  type?: string;
  description?: string;
  required?: boolean;
  enum?: string[];
  default?: string;
}

export interface OSTemplate {
  templateName: string;
  distribution: string;
  family: string;
  bitFormat: number;
  filesystems?: string[];
  lvmReady?: boolean;
  noPartitioning?: boolean;
  softRaidOnlyMirroring?: boolean;
  customizeQuestions?: CustomizeQuestion[];
}

export interface DiskGroupDisk {
  number: number;
  capacity: number;
  unit: string;
  technology?: string;
  interface?: string;
  diskType?: string;
}

export interface DiskGroup {
  raidController?: string;
  disks: DiskGroupDisk[];
  diskType?: string;
  description?: string;
  id?: number;
}

export interface PartitionScheme {
  name: string;
  priority: number;
}

export interface CustomPartition {
  mountpoint: string;
  filesystem: string;
  size: number;
  order: number;
  type: string;
  raid?: string;
  diskGroupId?: number;
  lvName?: string;
  zpoolName?: string;
}

export interface ReinstallArgs {
  serviceName: string;
  templateName: string;
  customHostname?: string;
  customizations?: Record<string, string | boolean>;
  keepDiskGroups?: number[];
  softwareRaidDisks?: number;
  hwRaidArrays?: number;
  hwRaidSpares?: number;
  useProxmox9Zfs?: boolean;
  zfsRaidLevel?: 0 | 1;
  zfsVzSize?: number;
  partitionSchemeName?: string;
  hardwareRaid?: Record<number, string>;
  softwareRaidLevel?: string;
  useSoftwareRaid?: boolean;
  customPartitions?: CustomPartition[];
  diskGroups?: Record<string, DiskGroup>;
}

export interface ReinstallResult {
  success: boolean;
  message?: string;
  taskId?: number;
  warnings?: string[];
}

export interface InstallStep {
  comment: string;
  commentOriginal: string;
  status: string;
  error: string;
}

export interface InstallStatus {
  elapsedTime: number;
  progressPercentage: number;
  totalSteps: number;
  completedSteps: number;
  hasError: boolean;
  stopping?: boolean;
  allDone: boolean;
  progressUnknown: boolean;
  steps: InstallStep[];
}

export interface BackupFtpAccessEntry {
  ipBlock: string;
  ftp?: boolean;
  nfs?: boolean;
  cifs?: boolean;
  isApplied?: boolean;
  error?: string;
}

export interface BackupFtpResult {
  backupFtp?: Record<string, any> | null;
  accessList?: BackupFtpAccessEntry[];
  accessFailedCount?: number;
  accessError?: string;
  notAvailable?: boolean;
  notActivated?: boolean;
  unknownService?: boolean;
  error?: string;
  reason?: string;
}

export interface BackupCloudResult {
  backupCloud?: Record<string, any> | null;
  notAvailable?: boolean;
  notActivated?: boolean;
  unknownService?: boolean;
  error?: string;
  reason?: string;
}

export interface SecondaryDnsDomain extends DetailErrorMarked {
  domain: string;
  dns?: string;
  ipMaster?: string;
}

export interface VirtualMacEntry extends DetailErrorMarked {
  macAddress: string;
  type?: string;
  ipAddress?: string;
  virtualNetworkInterface?: string;
}

export interface VrackEntry extends DetailErrorMarked {
  vrackName: string;
  name?: string;
  description?: string;
}

export interface ServerOptionEntry extends DetailErrorMarked {
  option: string;
  state?: string;
  expirationDate?: string;
}

export interface FaultyDisk {
  disk_serial: string;
  slot_id?: number;
}

export interface ContactChangeRequestList {
  requests: any[];
  unsupported: boolean;
  message?: string;
  failedCount: number;
}

export interface RetractionInfo {
  eligible: boolean;
  reason?: string;
  message?: string;
  orderId?: number;
  orderUrl?: string;
  orderDate?: string;
  retractionDate?: string;
  hoursLeft?: number;
  reasons?: { value: string; label: string }[];
}

export interface RescueBoot {
  bootId: number;
  bootType?: string;
  kernel?: string;
  description?: string;
}

export interface RescueStatus {
  inRescue: boolean;
  currentBoot: number;
  rescueMail?: string;
  boots: RescueBoot[];
}

export interface SplaLicense {
  id: number;
  type: string;
  status: string;
  serialNumber?: string;
  lastUpdate?: string;
}

export interface SplaListResult {
  list: SplaLicense[];
  partial: boolean;
}
