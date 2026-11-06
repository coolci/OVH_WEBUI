# 部署、迁移与测试工具审计

> **2026-11-06修复更新：本报告确认项已完成代码修复与对应离线回归。** 方案见[FIXES](FIXES.md)，当前结果见[TESTS](TESTS.md)。以下是发现时记录，“待修复”、错误证明与旧日志仅描述修复前基线。TL-01历史凭据外部轮换未执行。

## 发现时记录（历史）

这些脚本属于仓库范围，不能因未被 WebUI 导入而忽略。只进行了源码审查、AST提取和离线片段执行；没有运行远程部署、SSH连接或带真实凭据的烟测。下列7项为本轮补充：1项P1、5项P2、1项P3，均待修复。

## 独立逻辑清单

| 文件/入口 | 独立流程与判断 |
|---|---|
| backend/docker-entrypoint.sh及scratch副本 | 检查API密钥占位值→创建数据目录→exec后端；主部署配置见platform.md |
| scripts/init-first-run.ps1 / .sh | 随机API密钥、环境文件生成、可选数据目录备份重置；是真实文件写入工具，本次未执行 |
| scripts/start-backend.ps1 | 探活、可选编译、停止旧进程、隐藏启动、轮询健康；TL-05；按进程名停止可能影响其它工作副本，列操作风险 |
| scripts/start-dev.ps1 | 后端启动→安装依赖→Vite公开监听；开发用途，不视作生产认证边界 |
| scripts/docker-deploy.sh | 参数决定up/down/logs/build，生成.env并调用Compose；up成功只证明容器启动命令完成 |
| scripts/linux-oneclick-deploy.sh | 密钥与DB key生成、环境加载、域名设置、HTTP/HTTPS栈切换、健康等待；等待超时只告警仍打印完成，属于TL-02同族 |
| scratch/package_and_deploy.py | 打包→SSH→备份→上传→构建→覆盖→重启→健康检查；TL-01/02；备份tar允许失败，未证明SQLite在线备份一致性 |
| scratch/deploy.sh | 固定发布包解压、镜像/数据备份、构建切换、版本记录、真实API检查；固定路径和发布号只适用于特定历史部署 |
| scratch/check_remote.py / check_server_compose.py | 固定绑定地址和目标SSH，读容器/仓库/日志；TL-01；AutoAddPolicy未验证初次主机指纹 |
| scripts/_port_pages.py | 旧路由剥离、页面外壳生成、导航API替换；TL-03，硬编码旧工作区路径 |
| scripts/_fix_router_imports.py / _patch_imports.py | 正则替换旧导入、删除Route声明、生成hook转导出；一次性迁移脚本，不是当前构建过程 |
| scripts/sync_upstream.cjs | git show→模块路径替换→写目标文件；参数拼入shell且目标允许越出工作区，只适合可信手工参数；无远程输入入口，不按已利用Web漏洞计数 |
| scratch/audit_api_routes.cjs | 扫main.go路由→扫前端路径→正则对照；TL-04，不可作为当前契约完整性证明 |
| scripts/check-encoding.py | 遍历文本、排除数据/密钥和二进制、检测非法编码/疑似乱码；报告只输出位置 |
| scripts/analyze_storage_cache.py | GET目录→存储介质分类→选项归一化碰撞→展示统计；是近似诊断，不是生产匹配器；样本上限不代表真实错误总数 |
| scripts/smoke_test.py | 健康、旧接口、账户/认证、目录、白名单机器详情、日志；可依据环境变量创建账户，不能当零写入测试 |
| scripts/full_functional_test.py | 分组检查系统/账户/目录/设备/询价/订阅更新及报告；允许特定软失败、部分跳过记PASS，TL-06；会创建临时购物车且对占位订阅发PUT |
| scripts/order-inputs.test.mjs | 转译真实输入辅助函数，13项归一化/自定义型号/多账户库存断言 |
| scripts/test-subscription-ui.cjs | 真实React页面+全网络mock；订阅账户选择、旧数据编辑、自定义型号及区域错误回归 |
| scratch_test.cjs | 对本地服务POST price命令；使用固定示例密钥，可能触发询价建车，本次未执行 |
| tools/status/main.go | CSV读取→契约文本/目录/测试文件存在性→done/wip统计→可选写回；TL-07；saveLedger未检查Flush/Close错误属待注入风险 |
| eslint/postcss/Vite/TypeScript配置 | 静态质量规则、样式插件、开发代理、别名与构建；质量结果见TESTS.md |

## 确认问题

### TL-01 · P1 · 受控脚本硬编码SSH密码

- 位置：`scratch/check_remote.py:11`、`scratch/check_server_compose.py:10`、`scratch/package_and_deploy.py:11`（在`:41`用于SSH连接）。
- 证据：AST确认非空字符串被传入password参数/密码变量；三个文件均在git受控清单内。报告及证明输出不复制密码值，未测试该凭据是否仍有效。
- 影响：仓库读取权限可能扩展为服务器访问能力；删除工作树中的字符串不能消除已有提交历史。
- 建议：由持有者轮换该凭据、审查访问记录；改用SSH agent或环境/密钥存储，验证主机指纹；再按仓库协作流程清理历史和镜像。审计没有擅自连接服务器或轮换凭据。

### TL-02 · P2 · 部署健康检查失败仍返回成功

- 位置：`scratch/package_and_deploy.py:200–217`。循环15次后没有失败出口，最后docker ps成功即让run_cmd通过，随后打印部署成功。
- 证据：从Python AST提取真实verify_script，仅替换docker/sleep为本地函数；连续15次unhealthy，退出码仍为0，没有出现healthy分支。
- 同族：`scripts/linux-oneclick-deploy.sh:211–245`超时仅告警，仍打印“部署完成”；未实际部署验证。
- 建议：显式健康结果，超时非零退出；必要时回滚并保留失败日志。成功/超时/容器不存在分别回归。

### TL-03 · P2 · 旧页面迁移脚本生成非法导航代码

- 位置：`scripts/_port_pages.py:105–107`，先字符串删除`{ to:`，后面的正则已经匹配不到原形式。
- 证据：执行源码中这两次真实变换，`navigate({ to: "/queue" })`变成`navigate("/queue" })`，多余闭合大括号保留。
- 影响：重新执行迁移会破坏生成代码；当前构建不调用它，不代表当前线上页面已损坏。
- 建议：直接使用完整模式替换或AST迁移；在临时目录生成后先typecheck再替换源码。

### TL-04 · P3 · 旧接口审计工具得到零条后端路由

- 位置：`scratch/audit_api_routes.cjs:4–17`只读main.go，实际注册已移到internal/router。
- 证据：离线执行输出`Found 0 backend routes`，实际构造Gin router得到254条。后续“True unmapped”不能当成真实缺失接口列表。
- 建议：使用本报告实际路由导出方法；校验非零条数、HTTP method和动态参数后再对照。

### TL-05 · P2 · 在线后端使-Rebuild静默失效

- 位置：`scripts/start-backend.ps1:31–35`探活成功立即exit 0；`:47`之后才检查Rebuild。
- 证据等级：确定性静态控制流。传-Rebuild且探活200时必定跳过编译；未运行脚本或停止用户进程。
- 影响：`npm run backend:rebuild`在最常见的服务已运行状态下仍使用旧二进制。
- 建议：提前处理Rebuild，编译成功后再切换指定路径的进程；验证运行/未运行、编译失败、重启失败的退出码与版本。

### TL-06 · P2 · 全功能测试把HTTP500/401错误视为通过

- 位置：`scripts/full_functional_test.py`的`ok_soft`（约352行）接受任意含error或success=false的字典，不限制HTTP码。
- 证据：只提取并执行真实函数，输入500+error及401+error均返回True/soft-ok。
- 影响：上游宕机、鉴权失败可能被归为“功能未开通”，全功能测试绿灯不能证明这些接口可用。
- 建议：只允许明确能力缺失码；401/403/429/5xx必须独立失败或明确降级，SKIP与PASS分开统计。报告保留原始HTTP码。

### TL-07 · P2 · ledger验证不验证HTTP方法、真实实现或测试通过

- 位置：`tools/status/main.go`的deriveStatus、hasLayeredImplementation、hasPassingTest；后两者忽略target，只判断领域目录/测试文件存在，HTTP method完全未参与检查。
- 证据：临时复制一条现有登记，将method改为`NOT_A_METHOD`且status为done，运行真实`--verify`仍以0退出并报告1/1 done。未改仓库ledger，见[ledger-false-positive.log](evidence/ledger-false-positive.log)。
- 影响：verify:ledger的100%只能说明文本和目录存在，不能证明接口实现或测试正确；这也解释登记228条而实际路由254条仍全绿。
- 建议：使用真实router的method+规范化path对照，接口测试与路径显式关联，测试执行结果单独作为门禁。现有API OpenAPI文本不能只用子串存在作契约证明。

## 证明与限制

运行`python docs/audit-2026-11-06/proofs/tooling-proof.py`，输出见[tooling-results.log](evidence/tooling-results.log)。TL-01为AST直接证据，TL-05为静态分支证据，其余为真实源码/片段隔离执行。脚本中的其它潜在操作风险没有冒充动态复现BUG。
