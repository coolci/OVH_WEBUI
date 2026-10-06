package handlers

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/numconv"
)

// ============================================================================
// 硬件"中奖"检测
//
// 定义:下单时订购的配置 ≠ 机房实际交付的硬件(且实际更好),社区俗称"中奖"。
//
// 订购配置来源(账单侧,下单那一刻就定死,之后不会变):
//   GET /services/{serviceId}          → resource.product.description = 订购 CPU(如 "Intel Xeon E5-1620v2")
//   GET /services/{serviceId}/options  → 子服务里 ram-32g-ecc-1333 / softraid-2x480ssd 就是订购的内存 / 硬盘
// 实际配置来源(机房侧):
//   GET /dedicated/server/{sn}/specifications/hardware → processorName / memorySize / diskGroups
//
// 实例(ns392029 KS-LE-C):订购 E5-1620v2 + 32GB,实配 E5-1630v4 + 64GB → CPU、内存双中奖。
// 注意 specifications/hardware 里的 description("KS-LE-C - Intel Xeon E5-1620v2")也是订购侧文案,
// 不是实配,不能拿它当实际 CPU。
// ============================================================================

// hwLotteryItem 单项中奖明细(只列出"实际优于订购"的项)
type hwLotteryItem struct {
	Kind    string `json:"kind"`    // cpu | memory | disk
	Ordered string `json:"ordered"` // 订购配置展示文案
	Actual  string `json:"actual"`  // 实际配置展示文案
}

// hwLottery 返回给前端的比对结果
type hwLottery struct {
	// Checked=false 表示没拿到订购配置(权限不足 / 老合同没有 options 等),前端不显示任何中奖标识
	Checked  bool            `json:"checked"`
	Won      bool            `json:"won"`
	PlanCode string          `json:"planCode,omitempty"`
	PlanName string          `json:"planName,omitempty"`
	Items    []hwLotteryItem `json:"items"`
	Reason   string          `json:"reason,omitempty"`
}

// orderedSpec 从账单接口解析出的订购配置
type orderedSpec struct {
	PlanCode  string
	PlanName  string
	CPU       string // 订购 CPU 文案,可能为空
	MemoryGB  float64
	MemLabel  string // 如 "32GB DDR3 ECC 1333MHz"
	Disks     []diskSpec
	DiskLabel string
}

type diskSpec struct {
	Count  int
	SizeGB float64
	Type   string // nvme | ssd | sa | hdd ...
}

// svcLite /services/{id} 与 /services/{id}/options 共用的精简结构
type svcLite struct {
	Billing struct {
		Plan struct {
			Code        string `json:"code"`
			InvoiceName string `json:"invoiceName"`
		} `json:"plan"`
	} `json:"billing"`
	Resource struct {
		Product struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"product"`
	} `json:"resource"`
}

// 订购配置下单后不会再变,按 serviceName 缓存,避免每次打开概览都多打 3 个 OVH 请求
var orderedSpecCache sync.Map // serviceName -> *orderedSpec

var (
	reRAM  = regexp.MustCompile(`^ram-(\d+)g`)
	reDisk = regexp.MustCompile(`(\d+)x(\d+)(nvme|ssd|sas|sa|hdd)`)
	// CPU 名里去掉品牌/系列词,只留型号核心,便于 "Intel Xeon E-2274G" 与 "XeonE-2274G" 判等
	reCPUNoise = regexp.MustCompile(`(?i)\(r\)|\(tm\)|intel|xeon|amd|ryzen|epyc|core|processor|cpu`)
	reNonAlnum = regexp.MustCompile(`[^a-z0-9]`)
	reHasDigit = regexp.MustCompile(`\d`)
)

// fetchOrderedSpec 拉订购配置(带缓存)
func fetchOrderedSpec(client *ovhsdk.Client, svc string) (*orderedSpec, error) {
	if v, ok := orderedSpecCache.Load(svc); ok {
		return v.(*orderedSpec), nil
	}
	sid, err := serviceIDForDedicated(client, svc)
	if err != nil {
		return nil, err
	}
	var (
		main       svcLite
		opts       []svcLite
		mErr, oErr error
		wg         sync.WaitGroup
	)
	wg.Add(2)
	go func() { defer wg.Done(); mErr = client.Get(fmt.Sprintf("/services/%d", sid), &main) }()
	go func() { defer wg.Done(); oErr = client.Get(fmt.Sprintf("/services/%d/options", sid), &opts) }()
	wg.Wait()
	if mErr != nil {
		return nil, mErr
	}
	if oErr != nil {
		return nil, oErr
	}
	spec := parseOrderedSpec(main, opts)
	orderedSpecCache.Store(svc, spec)
	return spec, nil
}

// parseOrderedSpec 纯函数:账单数据 → 订购配置
func parseOrderedSpec(main svcLite, opts []svcLite) *orderedSpec {
	spec := &orderedSpec{
		PlanCode: main.Billing.Plan.Code,
		PlanName: main.Billing.Plan.InvoiceName,
	}
	if d := strings.TrimSpace(main.Resource.Product.Description); reHasDigit.MatchString(d) {
		spec.CPU = d
	}
	for _, o := range opts {
		name := strings.ToLower(o.Resource.Product.Name)
		if name == "" {
			name = strings.ToLower(o.Billing.Plan.Code)
		}
		label := o.Billing.Plan.InvoiceName
		if label == "" {
			label = o.Resource.Product.Description
		}
		if m := reRAM.FindStringSubmatch(name); m != nil {
			gb, _ := strconv.ParseFloat(m[1], 64)
			spec.MemoryGB = gb
			spec.MemLabel = label
			continue
		}
		if strings.HasPrefix(name, "bandwidth") || strings.HasPrefix(name, "vrack") || strings.HasPrefix(name, "traffic") {
			continue
		}
		if ms := reDisk.FindAllStringSubmatch(name, -1); len(ms) > 0 {
			spec.Disks = spec.Disks[:0]
			for _, m := range ms {
				n, _ := strconv.Atoi(m[1])
				sz, _ := strconv.ParseFloat(m[2], 64)
				spec.Disks = append(spec.Disks, diskSpec{Count: n, SizeGB: sz, Type: m[3]})
			}
			spec.DiskLabel = label
		}
	}
	return spec
}

// computeHardwareLottery 纯函数:订购配置 vs 实际硬件
func computeHardwareLottery(spec *orderedSpec, hardware map[string]interface{}) hwLottery {
	res := hwLottery{Items: []hwLotteryItem{}}
	if spec == nil {
		res.Reason = "订购配置不可用"
		return res
	}
	res.Checked = true
	res.PlanCode = spec.PlanCode
	res.PlanName = spec.PlanName

	// ---- CPU:型号不同即视为中奖(OVH 只会往上替换,不会给更差的) ----
	orderedCPU := spec.CPU
	if orderedCPU == "" {
		// 兜底:hardware.description 形如 "KS-LE-C - Intel Xeon E5-1620v2",最后一段是订购 CPU
		if desc, _ := hardware["description"].(string); desc != "" {
			if i := strings.LastIndex(desc, " - "); i >= 0 {
				orderedCPU = strings.TrimSpace(desc[i+3:])
			}
		}
	}
	actualCPU, _ := hardware["processorName"].(string)
	if orderedCPU != "" && actualCPU != "" && !cpuSame(orderedCPU, actualCPU) {
		res.Items = append(res.Items, hwLotteryItem{Kind: "cpu", Ordered: orderedCPU, Actual: actualCPU})
	}

	// ---- 内存:实际 > 订购 ----
	if spec.MemoryGB > 0 {
		if actualGB := memoryGB(hardware["memorySize"]); actualGB > spec.MemoryGB {
			res.Items = append(res.Items, hwLotteryItem{
				Kind:    "memory",
				Ordered: fmtGB(spec.MemoryGB),
				Actual:  fmtGB(actualGB),
			})
		}
	}

	// ---- 硬盘:总容量多 5% 以上,或介质升级(HDD→SSD→NVMe) ----
	if len(spec.Disks) > 0 {
		groups, _ := hardware["diskGroups"].([]interface{})
		var actTotal float64
		actRank := 99
		var actParts []string
		for _, g := range groups {
			gm, _ := g.(map[string]interface{})
			if gm == nil {
				continue
			}
			n, _ := numconv.ToInt64(gm["numberOfDisks"])
			if n <= 0 {
				n = 1
			}
			sz := sizeGB(gm["diskSize"])
			typ, _ := gm["diskType"].(string)
			actTotal += float64(n) * sz
			if r := diskRank(typ); r < actRank {
				actRank = r
			}
			actParts = append(actParts, fmt.Sprintf("%d× %s %s", n, strings.ToUpper(typ), fmtGB(sz)))
		}
		var ordTotal float64
		ordRank := 99
		var ordParts []string
		for _, d := range spec.Disks {
			ordTotal += float64(d.Count) * d.SizeGB
			if r := diskRank(d.Type); r < ordRank {
				ordRank = r
			}
			ordParts = append(ordParts, fmt.Sprintf("%d× %s %s", d.Count, diskTypeLabel(d.Type), fmtGB(d.SizeGB)))
		}
		if len(actParts) > 0 {
			bigger := actTotal > ordTotal*1.05
			// 混合盘(多组)订购时介质比较意义不大,只比单组
			better := len(spec.Disks) == 1 && actRank != 99 && actRank > ordRank
			if bigger || better {
				res.Items = append(res.Items, hwLotteryItem{
					Kind:    "disk",
					Ordered: strings.Join(ordParts, " + "),
					Actual:  strings.Join(actParts, " + "),
				})
			}
		}
	}

	res.Won = len(res.Items) > 0
	return res
}

func normCPU(s string) string {
	s = reCPUNoise.ReplaceAllString(strings.ToLower(s), "")
	return reNonAlnum.ReplaceAllString(s, "")
}

func cpuSame(a, b string) bool {
	na, nb := normCPU(a), normCPU(b)
	if na == "" || nb == "" {
		return true // 解析不出型号就不下结论,宁可漏报也不误报
	}
	return na == nb || strings.Contains(na, nb) || strings.Contains(nb, na)
}

// memoryGB {value, unit} → GB
func memoryGB(v interface{}) float64 { return sizeGB(v) }

func sizeGB(v interface{}) float64 {
	m, _ := v.(map[string]interface{})
	if m == nil {
		return 0
	}
	val, _ := numconv.ToFloat64(m["value"])
	switch strings.ToUpper(fmt.Sprint(m["unit"])) {
	case "MB":
		return val / 1024
	case "TB":
		return val * 1024
	case "KB":
		return val / 1024 / 1024
	default:
		return val
	}
}

func fmtGB(gb float64) string {
	if gb >= 1024 && math.Mod(gb, 1024) == 0 {
		return fmt.Sprintf("%g TB", gb/1024)
	}
	return fmt.Sprintf("%g GB", math.Round(gb*10)/10)
}

func diskRank(t string) int {
	switch strings.ToLower(t) {
	case "nvme":
		return 2
	case "ssd":
		return 1
	case "":
		return 99
	default: // sa / sas / hdd / sata
		return 0
	}
}

func diskTypeLabel(t string) string {
	switch strings.ToLower(t) {
	case "nvme":
		return "NVME"
	case "ssd":
		return "SSD"
	default:
		return "HDD"
	}
}
