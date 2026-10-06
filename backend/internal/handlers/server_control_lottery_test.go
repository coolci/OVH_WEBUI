package handlers

import "testing"

func mkSvc(code, invoice, product, desc string) svcLite {
	var s svcLite
	s.Billing.Plan.Code = code
	s.Billing.Plan.InvoiceName = invoice
	s.Resource.Product.Name = product
	s.Resource.Product.Description = desc
	return s
}

func disk(n int, size float64, unit, typ string) map[string]interface{} {
	return map[string]interface{}{
		"numberOfDisks": float64(n),
		"diskSize":      map[string]interface{}{"value": size, "unit": unit},
		"diskType":      typ,
	}
}

// 以下数据取自真实 OVH 报文(用户账号下的 3 台机器)
func TestHardwareLottery_RealPayloads(t *testing.T) {
	// ns392029 KS-LE-C:订购 E5-1620v2 + 32GB,实配 E5-1630v4 + 64GB → CPU + 内存中奖
	won := parseOrderedSpec(
		mkSvc("26sklec01-v1", "KS-LE-C", "26sklec01", "Intel Xeon E5-1620v2"),
		[]svcLite{
			mkSvc("bandwidth-500-26skle", "500Mbps unmetered public bandwidth", "bandwidth-500", ""),
			mkSvc("softraid-2x480ssd-26sklec01-v1", "2x SSD SATA 480GB Enterprise Class Soft RAID", "softraid-2x480ssd", ""),
			mkSvc("ram-32g-ecc-1333-26sklec01-v1", "32GB DDR3 ECC 1333MHz", "ram-32g-ecc-1333", ""),
		})
	r := computeHardwareLottery(won, map[string]interface{}{
		"description":   "KS-LE-C - Intel Xeon E5-1620v2",
		"processorName": "E5-1630v4",
		"memorySize":    map[string]interface{}{"value": float64(65536), "unit": "MB"},
		"diskGroups":    []interface{}{disk(2, 480, "GB", "SSD")},
	})
	if !r.Checked || !r.Won || len(r.Items) != 2 {
		t.Fatalf("KS-LE-C 应为 CPU+内存中奖, got %+v", r)
	}
	if r.Items[0].Kind != "cpu" || r.Items[1].Kind != "memory" || r.Items[1].Ordered != "32 GB" || r.Items[1].Actual != "64 GB" {
		t.Fatalf("明细不对: %+v", r.Items)
	}

	// ns3200360 KS-5-A:完全一致("Intel Xeon E-2274G" vs "XeonE-2274G" 要判等)
	same := parseOrderedSpec(
		mkSvc("26sk50a-v1", "KS-5-A | Intel Xeon E-2274G", "26sk50a", "Intel Xeon E-2274G"),
		[]svcLite{
			mkSvc("ram-32g-ecc-2666-26sk50a-v1", "32GB DDR4 ECC 2666MHz", "ram-32g-ecc-2666", ""),
			mkSvc("softraid-2x960nvme-26sk50a-v1", "", "softraid-2x960nvme", ""),
		})
	r = computeHardwareLottery(same, map[string]interface{}{
		"processorName": "XeonE-2274G",
		"memorySize":    map[string]interface{}{"value": float64(32768), "unit": "MB"},
		"diskGroups":    []interface{}{disk(2, 960, "GB", "NVME")},
	})
	if !r.Checked || r.Won {
		t.Fatalf("KS-5-A 不应中奖, got %+v", r)
	}

	// ns529169 KS-LE-B:一致
	same2 := parseOrderedSpec(
		mkSvc("26skleb01-v1", "KS-LE-B | Intel Xeon E3-1245v5", "26skleb01", "Intel Xeon E3-1245v5"),
		[]svcLite{
			mkSvc("softraid-2x450nvme-26skleb01-v1", "", "softraid-2x450nvme", ""),
			mkSvc("ram-32g-ecc-2400-26skleb01-v1", "", "ram-32g-ecc-2400", ""),
		})
	r = computeHardwareLottery(same2, map[string]interface{}{
		"processorName": "E3-1245v5",
		"memorySize":    map[string]interface{}{"value": float64(32768), "unit": "MB"},
		"diskGroups":    []interface{}{disk(2, 450, "GB", "NVME")},
	})
	if r.Won {
		t.Fatalf("KS-LE-B 不应中奖, got %+v", r)
	}
}

func TestHardwareLottery_Disk(t *testing.T) {
	spec := parseOrderedSpec(mkSvc("p", "", "", ""), []svcLite{mkSvc("", "", "softraid-2x2000sa", "")})
	// 介质升级 HDD → SSD
	r := computeHardwareLottery(spec, map[string]interface{}{"diskGroups": []interface{}{disk(2, 2, "TB", "SSD")}})
	if !r.Won || r.Items[0].Kind != "disk" {
		t.Fatalf("HDD→SSD 应中奖, got %+v", r)
	}
	// 容量一致不中奖
	r = computeHardwareLottery(spec, map[string]interface{}{"diskGroups": []interface{}{disk(2, 2000, "GB", "HDD")}})
	if r.Won {
		t.Fatalf("容量介质一致不应中奖, got %+v", r)
	}
}

func TestHardwareLottery_NoSpec(t *testing.T) {
	r := computeHardwareLottery(nil, map[string]interface{}{})
	if r.Checked || r.Won || r.Items == nil {
		t.Fatalf("无订购配置时应 checked=false 且 items 非 nil, got %+v", r)
	}
}
