package catalog

import (
	"regexp"
	"strings"
)

var modelPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-\d+skl[a-e]\d{2}(-v\d+)?`),
	regexp.MustCompile(`-\d+sk\d+`),
	regexp.MustCompile(`-\d+rise\d*`),
	regexp.MustCompile(`-\d+sys\w*`),
	regexp.MustCompile(`-\d+risegame\d*`),
	regexp.MustCompile(`-\d+risestor`),
	regexp.MustCompile(`-\d+skgame\d*`),
	regexp.MustCompile(`-\d+ska\d*`),
	regexp.MustCompile(`-\d+skstor\d*`),
	regexp.MustCompile(`-\d+sysstor`),
	regexp.MustCompile(`game\d*`),
	regexp.MustCompile(`stor\d*`),
	regexp.MustCompile(`-ks\d+`),
	regexp.MustCompile(`-rise`),
	regexp.MustCompile(`-\d+sysle\d+`),
	regexp.MustCompile(`-\d+skb\d+`),
	regexp.MustCompile(`-\d+skc\d+`),
	regexp.MustCompile(`-\d+sk\d+b`),
	regexp.MustCompile(`-v\d+`),
	regexp.MustCompile(`-[a-z]{3}$`),
}

var (
	reEcc = regexp.MustCompile(`-(no)?ecc-\d+`)
	// sas 必须在 sa 前，避免 -sas 被当成 -sa
	reStorSfx   = regexp.MustCompile(`-(sas|ssd|nvme|hdd|sa)$`)
	reSpecDigit = regexp.MustCompile(`-\d{4,5}$`)
)

// StandardizeConfig 对应 Python: standardize_config
func StandardizeConfig(config string) string {
	if config == "" {
		return ""
	}
	normalized := strings.TrimSpace(strings.ToLower(config))
	for _, p := range modelPatterns {
		normalized = p.ReplaceAllString(normalized, "")
	}
	normalized = reEcc.ReplaceAllString(normalized, "")
	normalized = reStorSfx.ReplaceAllString(normalized, "")
	normalized = reSpecDigit.ReplaceAllString(normalized, "")
	return normalized
}

// FormatMemoryDisplay 对应 Python: format_memory_display
func FormatMemoryDisplay(memoryCode string) string {
	if m := regexp.MustCompile(`(?i)(\d+)g`).FindStringSubmatch(memoryCode); m != nil {
		return m[1] + "GB RAM"
	}
	return memoryCode
}

// FormatStorageDisplay 短码 / addon → 可读存储
func FormatStorageDisplay(storageCode string) string {
	if storageCode == "" {
		return ""
	}
	// 完整 addon
	if disp := ParseStorageAddonDisplay(storageCode); disp != "" {
		// 短码 2x480ssd：Parse 会加 SOFTRAID 前缀，短码场景只返回 segment
		if !strings.Contains(strings.ToLower(storageCode), "raid") &&
			!strings.Contains(strings.ToLower(storageCode), "disk") {
			if m := reStorageSegment.FindStringSubmatch(storageCode); m != nil {
				return formatStorageSegment(m[1], m[2], m[3])
			}
		}
		return disp
	}
	if m := reStorageSegment.FindStringSubmatch(storageCode); m != nil {
		return formatStorageSegment(m[1], m[2], m[3])
	}
	return storageCode
}

// FormatConfigDisplay 对应 Python: format_config_display
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

// MatchConfig 对应 Python: match_config
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
