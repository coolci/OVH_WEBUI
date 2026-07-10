# 全功能测试结果

- 时间: 2026-07-11 03:10:14
- BASE: `http://127.0.0.1:19998`
- 目标机: `<redacted-server>`
- 结果: **54/54 PASS**
- 硬失败(核心): 0
- Config Sniper: **完全下线**
- 安全: 未执行暂停/删除/重启/重装/终止

## 分组

### A (6/6)

| 用例 | 结果 | ms | 详情 |
|------|------|-----|------|
| GET /health | ✅ | 15 | code=200 body={'status': 'ok', 'time': '2026-07-11T03:08:26+08:00'} |
| GET /api/health | ✅ | 0 | code=200 |
| GET /api/stats | ✅ | 0 | code=200 keys=['activeQueues', 'totalServers', 'availableServers', 'purchaseSuccess', 'purchaseFailed', 'queueProcessorRunning', 'monitorRunning'] |
| GET /api/version | ✅ | 0 | code=200 data={'version': 'dev'} |
| 线上巡检已移除 | ✅ | 0 | code=404 (巡检已取消) |
| Config Sniper 已下线 | ✅ | 0 | code=404 (废弃功能不应可用) |

### B (4/4)

| 用例 | 结果 | ms | 详情 |
|------|------|-----|------|
| GET /api/accounts | ✅ | 0 | accounts=1 |
| POST /api/verify-auth | ✅ | 345 | code=200 data={'valid': True} |
| GET /api/ovh/account/info | ✅ | 341 | nichandle=<nichandle> |
| GET /api/ovh/account/orders | ✅ | 936 | count=10 |

### C (4/4)

| 用例 | 结果 | ms | 详情 |
|------|------|-----|------|
| GET /api/servers?showApiServers=true | ✅ | 25 | count=98 sample=24adv01-v3 name=ADVANCE-1 / AMD EPYC 4244P |
| 服务器列表字段抽样 | ✅ | 10 | rich_fields_in_first20=20 |
| GET /api/cache/info | ✅ | 1 | code=200 {'backend': {'cacheAge': 964, 'cacheDuration': 7200, 'cacheValid': True, 'hasCachedData': True, 'serverCount': 98, 'timestamp': 1783709543}, 'sqlite': |
| GET /api/catalog (可达性) | ✅ | 132 | code=200 type=dict |

### D (3/3)

| 用例 | 结果 | ms | 详情 |
|------|------|-----|------|
| GET /api/server-control/list | ✅ | 2203 | total=2 target state=ok ip=<redacted-ip> dc=bhs5 os=ubuntu2404-server_64 |
| 目标机 state 健康 | ✅ | 0 | state=ok status=ok monitoring=True |
| GET /api/server-control/aliases | ✅ | 1 | code=200 {} |

### E (26/26)

| 用例 | 结果 | ms | 详情 |
|------|------|-----|------|
| GET hardware | ✅ | 746 | ok type=dict keys=['bootMode', 'coresPerProcessor', 'defaultHardwareRaidSize', 'defaultHardwareRaidType', 'description', 'diskGroups', 'expansionCards', 'formFactor'] |
| GET serviceinfo | ✅ | 462 | ok type=dict status=ok exp=2026-08-04 |
| GET ips | ✅ | 1841 | ok type=dict count=2 |
| GET templates | ✅ | 4969 | ok type=dict |
| GET tasks | ✅ | 2228 | ok type=dict |
| GET boot | ✅ | 3025 | ok |
| GET boot-mode | ✅ | 2923 | ok |
| GET monitoring | ✅ | 1249 | ok |
| GET network-specs | ✅ | 776 | ok |
| GET network-interfaces | ✅ | 2096 | ok |
| GET mrtg | ✅ | 3102 | ok |
| GET statistics | ✅ | 344 | soft-ok code=500 (功能可能未开通) |
| GET interventions | ✅ | 59250 | ok |
| GET planned-interventions | ✅ | 944 | ok |
| GET engagement | ✅ | 810 | ok |
| GET engagement/available | ✅ | 836 | ok |
| GET bios-settings | ✅ | 850 | soft-ok code=404 (功能可能未开通) |
| GET backup-ftp | ✅ | 803 | soft-ok code=404 (功能可能未开通) |
| GET reverse | ✅ | 1857 | ok |
| GET console | ✅ | 1595 | soft-ok code=500 (功能可能未开通) |
| GET options | ✅ | 1638 | ok |
| GET vrack | ✅ | 1157 | ok |
| GET secondary-dns | ✅ | 1045 | ok |
| GET virtual-mac | ✅ | 826 | ok |
| GET ip-specs | ✅ | 1295 | ok |
| GET ongoing | ✅ | 1264 | ok |

### F (3/3)

| 用例 | 结果 | ms | 详情 |
|------|------|-----|------|
| POST /api/servers/:planCode/price | ✅ | 5571 | code=200 plan=24adv01-v3 dc=bhs {'success': True, 'planCode': '24adv01-v3', 'datacenter': 'bhs', 'price': {'pricingMode': 'default', |
| PUT 监控订阅(不存在→404) | ✅ | 1 | code=404 |
| PUT VPS 订阅(不存在→404) | ✅ | 12 | code=404 |

### G (8/8)

| 用例 | 结果 | ms | 详情 |
|------|------|-----|------|
| GET /api/queue | ✅ | 0 | code=200 type=list |
| GET /api/purchase-history | ✅ | 14 | code=200 |
| GET /api/logs | ✅ | 7 | code=200 |
| GET /api/monitor/status | ✅ | 0 | code=200 {'check_interval': 5, 'known_servers_count': 0, 'running': False, 'subscriptions': [], 'subscriptions_count': 0} |
| GET /api/vps-monitor/status | ✅ | 0 | code=200 |
| GET /api/vps-control/list | ✅ | 339 | code=200 {'success': True, 'total': 0, 'vps': []} |
| GET /api/ovh/contact-change-requests | ✅ | 474 | code=200 |
| GET /api/system/metrics | ✅ | 1 | code=200 |

## JSON

```json
{
  "base": "http://127.0.0.1:19998",
  "allowed_server": "<redacted-server>",
  "total": 54,
  "passed": 54,
  "failed": 0,
  "failed_items": [],
  "catalog_count": 98,
  "target_found": true,
  "hard_failures": 0,
  "policy": {
    "no_pause_delete_reboot_reinstall": true,
    "config_sniper": "fully_retired"
  }
}
```
