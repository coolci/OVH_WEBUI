package catalogwatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/ovh"
)

type Candidate struct {
	PlanCode    string   `json:"planCode"`
	Options     []string `json:"options"`
	Datacenters []string `json:"datacenters"`
}
type Family struct {
	Name      string   `json:"name"`
	Addons    []string `json:"addons"`
	Exclusive *bool    `json:"exclusive"`
	Mandatory *bool    `json:"mandatory"`
}
type Catalog struct {
	Plans []struct {
		PlanCode       string   `json:"planCode"`
		Families       []Family `json:"addonFamilies"`
		Configurations []struct {
			Name   string   `json:"name"`
			Values []string `json:"values"`
		} `json:"configurations"`
	} `json:"plans"`
}

func unique(values []string) []string {
	m := map[string]bool{}
	for _, v := range values {
		if v != "" {
			m[v] = true
		}
	}
	out := make([]string, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
func key(account string, c Candidate) string {
	b, _ := json.Marshal([]interface{}{account, c.PlanCode, unique(c.Options)})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Expand only catalog-declared choices. Datacenters are alternatives, not separate purchase groups.
func Expand(raw Catalog) (map[string]Candidate, error) {
	if len(raw.Plans) == 0 {
		return nil, fmt.Errorf("目录为空，保留原基线")
	}
	out := map[string]Candidate{}
	seen := map[string]bool{}
	for _, p := range raw.Plans {
		if p.PlanCode == "" || strings.ContainsAny(p.PlanCode, " ,，\t\r\n") || seen[p.PlanCode] {
			return nil, fmt.Errorf("目录型号格式异常")
		}
		seen[p.PlanCode] = true
		dcs := []string{}
		for _, c := range p.Configurations {
			if c.Name == "dedicated_datacenter" {
				dcs = append(dcs, c.Values...)
			}
		}
		dcs = unique(dcs)
		if len(dcs) == 0 {
			return nil, fmt.Errorf("%s 缺少可选机房，保留原基线", p.PlanCode)
		}
		combos := [][]string{{}}
		names := map[string]bool{}
		for _, f := range p.Families {
			if f.Name == "" || names[f.Name] {
				return nil, fmt.Errorf("配置分组异常")
			}
			names[f.Name] = true
			// Optional software licensing is not a server hardware configuration.
			// Enumerating license subsets creates billions of nonsensical duplicate machines.
			if f.Name == "application-license" || f.Name == "distribution-license" {
				if f.Mandatory != nil && *f.Mandatory {
					return nil, fmt.Errorf("%s 要求选择软件许可证，自动下单暂不支持", p.PlanCode)
				}
				continue
			}
			required := f.Mandatory == nil || *f.Mandatory
			exclusive := f.Exclusive == nil || *f.Exclusive
			choices := [][]string{}
			addons := unique(f.Addons)
			if exclusive {
				if !required {
					choices = append(choices, []string{})
				}
				for _, a := range addons {
					choices = append(choices, []string{a})
				}
			} else {
				if len(addons) > 13 {
					return nil, fmt.Errorf("%s 非互斥组合过多", p.PlanCode)
				}
				start := 0
				if required {
					start = 1
				}
				for bits := start; bits < (1 << len(addons)); bits++ {
					v := []string{}
					for i, a := range addons {
						if bits&(1<<i) != 0 {
							v = append(v, a)
						}
					}
					choices = append(choices, v)
				}
			}
			if len(choices) == 0 || len(combos)*len(choices) > 10000 {
				return nil, fmt.Errorf("%s 配置为空或超过 10000 组合，保留原基线", p.PlanCode)
			}
			next := [][]string{}
			for _, c := range combos {
				for _, v := range choices {
					next = append(next, append(append([]string{}, c...), v...))
				}
			}
			combos = next
		}
		for _, v := range combos {
			c := Candidate{p.PlanCode, unique(v), dcs}
			out[key("", c)] = c
		}
		if len(out) > 200000 {
			return nil, fmt.Errorf("目录组合总数异常")
		}
	}
	return out, nil
}
func fetch(ctx context.Context, state *app.State, sub string) (map[string]Candidate, error) {
	u := ovh.CatalogBaseURLForSubsidiary(sub) + "/v1/order/catalog/public/eco?" + url.Values{"ovhSubsidiary": {sub}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	res, err := netfp.Shared(20 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("官方目录 HTTP %d", res.StatusCode)
	}
	var raw Catalog
	if err = json.NewDecoder(io.LimitReader(res.Body, 64<<20)).Decode(&raw); err != nil {
		return nil, err
	}
	return Expand(raw)
}
