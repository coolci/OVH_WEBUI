package catalog

import (
	"regexp"
	"strings"
)

var (
	reTrailingRegion  = regexp.MustCompile(`-(us|ca|eu|ap|sgp|syd|ynm|mum|bhs|gra|rbx|sbg|waw|fra|lon|par|tor|yyz|vin|hil|lim|eri|mil)$`)
	reTrailingVersion = regexp.MustCompile(`-v\d+$`)
	reTrailingModel   = regexp.MustCompile(`-(\d{2}(sk|sys|rise|adv)[a-z0-9]*|ks-[a-z0-9]+(-[a-z0-9]+)*|ks\d+[a-z]*|rise|sys|sk|adv)$`)
	reEcc             = regexp.MustCompile(`-(no)?ecc-\d+`)
	reRamFreq         = regexp.MustCompile(`-\d{4,5}$`)
	reMediaHyphen     = regexp.MustCompile(`(\d+x\d+)-(nvme|ssd|sas|hdd|sata|sa)\b`)
	reDiskCountCap    = regexp.MustCompile(`^\d+x\d+([a-z]+)?$`)
	reMemOrCapToken   = regexp.MustCompile(`^\d+(g|gb|tb|mb|m)$`)
	reGenToken        = regexp.MustCompile(`^gen\d+$`)
)

var hardwareTokens = map[string]bool{
	"nvme": true, "ssd": true, "hdd": true, "sas": true, "sa": true, "sata": true,
	"0disk": true, "hybrid": true, "hybridsoftraid": true, "softraid": true,
	"hardraid": true, "raid": true, "gpu": true, "ram": true, "ecc": true,
	"noecc": true, "pcie": true, "u2": true, "m2": true, "system": true,
}

// IsAllowedOptionSuffix 判断 addon 码去掉前缀 "wanted-" 后的剩余部分是否仅为机型/版本/区域后缀，
// 拒绝包含额外硬盘组（如 2x6000sa）、容量或存储介质（如 nvme/ssd/hdd/sas）的硬件词段。
func IsAllowedOptionSuffix(rem string) bool {
	rem = strings.ToLower(strings.TrimSpace(rem))
	if rem == "" {
		return false
	}
	for _, tok := range strings.Split(rem, "-") {
		if tok == "" {
			return false
		}
		if hardwareTokens[tok] || reDiskCountCap.MatchString(tok) || reStorageSegment.MatchString(tok) ||
			reMemOrCapToken.MatchString(tok) || reGenToken.MatchString(tok) {
			return false
		}
	}
	return true
}

func stripTrailingModelAndRegion(s string) string {
	for {
		prev := s
		s = reTrailingRegion.ReplaceAllString(s, "")
		s = reTrailingVersion.ReplaceAllString(s, "")
		s = reTrailingModel.ReplaceAllString(s, "")
		if s == prev {
			return s
		}
	}
}

func StandardizeConfig(config string) string {
	if config == "" {
		return ""
	}
	normalized := strings.TrimSpace(strings.ToLower(config))
	normalized = stripTrailingModelAndRegion(normalized)
	normalized = reEcc.ReplaceAllString(normalized, "")
	if strings.HasPrefix(normalized, "ram-") {
		normalized = reRamFreq.ReplaceAllString(normalized, "")
	}
	// 统一同介质连接符（如 2x960-nvme → 2x960nvme），同时严格保留 nvme/ssd/hdd/sas/sa 介质差异
	normalized = reMediaHyphen.ReplaceAllString(normalized, "${1}${2}")
	return normalized
}

func FormatMemoryDisplay(memoryCode string) string {
	if m := regexp.MustCompile(`(?i)(\d+)g`).FindStringSubmatch(memoryCode); m != nil {
		return m[1] + "GB RAM"
	}
	return memoryCode
}

func FormatStorageDisplay(storageCode string) string {
	// 复用 storage_parse 的介质匹配（含 sas/sa），文案统一走 FormatMediaLabel（NVMe/SATA/SAS）
	if m := reStorageSegment.FindStringSubmatch(storageCode); m != nil {
		return formatStorageSegment(m[1], m[2], m[3])
	}
	return storageCode
}

func FormatConfigDisplay(memoryCode, storageCode string) string {
	mem := "默认内存"
	if memoryCode != "" {
		mem = FormatMemoryDisplay(memoryCode)
	}
	stor := "默认存储"
	if storageCode != "" {
		stor = FormatStorageDisplay(storageCode)
	}
	return mem + " + " + stor
}

func MatchConfig(userMemory, userStorage, ovhMemory, ovhStorage string) bool {
	memoryMatch := true
	if userMemory != "" && ovhMemory != "" {
		memoryMatch = StandardizeConfig(userMemory) == StandardizeConfig(ovhMemory)
	}
	storageMatch := true
	if userStorage != "" && ovhStorage != "" {
		storageMatch = StandardizeConfig(userStorage) == StandardizeConfig(ovhStorage)
	}
	return memoryMatch && storageMatch
}
