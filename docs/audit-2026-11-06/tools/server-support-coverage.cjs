const fs = require('node:fs');
const path = require('node:path');
const root = process.cwd();
const files = new Set();
function add(file) { if (fs.existsSync(file)) files.add(file.replaceAll('\\','/')); }
for (const file of fs.readdirSync('backend/internal/handlers')) {
  if (!file.endsWith('_test.go') && /^(server_control|vps_control|server_aliases|service_billing|termination_policy|account\.go|account_payment|support_tickets|support_chat)/.test(file)) add('backend/internal/handlers/'+file);
}
for(const f of ['backend/internal/router/routes_server_control.go','backend/internal/router/routes_vps_control.go','backend/internal/domain/server/server.go','backend/internal/service/server/server_service.go','backend/internal/handlers/util.go']) add(f);
function walk(dir) { for(const e of fs.readdirSync(dir, {withFileTypes:true})) { const p=dir+'/'+e.name; if(e.isDirectory()) walk(p); else if(/\.(ts|tsx)$/.test(e.name)) add(p); } }
for(const d of ['src/components/server-control','src/components/vps-control','src/components/tickets','src/components/support','src/components/account','src/hooks/ovh/server-control']) walk(d);
for(const p of ['ServerControlPage','VpsControlPage','AccountPage','TicketsPage']) add(`src/pages/${p}.tsx`);
for(const p of ['use-vps-control','use-server-control','use-account','use-tickets','use-account-services','use-active-account']) add(`src/hooks/ovh/${p}.ts`);
add('src/lib/http.ts');add('src/lib/query.ts');
add('backend/internal/handlers/order_mapping.go');add('src/hooks/ovh/use-mrtg.ts');add('src/hooks/use-mrtg.ts');
const full = new Set([
 ...[...files].filter(f=> f.startsWith('backend/') && !['backend/internal/handlers/account.go','backend/internal/handlers/util.go'].includes(f)),
 ...[...files].filter(f=> f.startsWith('src/hooks/ovh/server-control/use-')),
 'src/hooks/ovh/use-vps-control.ts','src/hooks/ovh/use-account.ts','src/hooks/ovh/use-tickets.ts','src/hooks/ovh/use-account-services.ts','src/hooks/ovh/use-active-account.ts','src/lib/query.ts',
 'src/pages/ServerControlPage.tsx','src/pages/VpsControlPage.tsx','src/pages/TicketsPage.tsx','src/pages/AccountPage.tsx',
 'src/components/support/OfficialChat.tsx','src/components/account/OrderPaymentButton.tsx',
 'src/components/vps-control/VpsDialogs.tsx','src/components/vps-control/VpsSnapshotPane.tsx','src/components/vps-control/VpsReinstallDialog.tsx','src/components/vps-control/VpsMitigationPane.tsx',
 'src/components/tickets/TicketList.tsx','src/components/server-control/EngagementDialog.tsx',
 ...['RescueDialog','RetractionDialog','RenewalDialog','PowerTab','ReinstallDialog','InstallProgressDialog','BootModeDialog'].map(p=>`src/components/server-control/${p}.tsx`),
 'src/components/server-control/reinstall/PostInstallSection.tsx','src/components/vps-control/VpsDetail.tsx',
 ...['HardwareReplaceDialog','TimeslotsDialog','ChangeContactDialog','IpmiDialog','AdvancedTab','BiosDialog','SplaDialog','ReverseDnsDialog','MaintenanceTab','TasksDialog'].map(p=>`src/components/server-control/${p}.tsx`),
 ...['OsTemplateSection','PartitionRow','SmartPlanDialog','StorageModeOption','StoragePartitionSection'].map(p=>`src/components/server-control/reinstall/${p}.tsx`),
 'src/components/server-control/reinstall/constants.ts',
 ...['VpsReverseDnsDialog','VpsSecondaryDnsPane','VpsOptionsPanel','VpsTasksDialog'].map(p=>`src/components/vps-control/${p}.tsx`),
 'src/components/tickets/TicketDetails.tsx','src/components/tickets/labels.ts',
 ...['OverviewTab','MrtgTrafficChart','NetworkSpecsDialog','LotteryConfetti','OsIcon'].map(p=>`src/components/server-control/${p}.tsx`),
 'src/components/tickets/CreateTicketDialog.tsx','src/components/tickets/TicketChat.tsx',
 'src/components/vps-control/vps-geo.ts','src/hooks/ovh/server-control/types.ts','src/hooks/ovh/use-server-control.ts',
 'src/lib/http.ts','src/hooks/ovh/use-mrtg.ts','src/hooks/use-mrtg.ts',
]);
const partial = {
 'backend/internal/handlers/account.go': {ranges:'1-428,650-EOF',teamReview:'full',teamRanges:[{reviewer:'audit_server_support',ranges:'1-428,650-EOF'},{reviewer:'audit_platform',ranges:'429-649'}],note:'区间并集1-EOF，无遗漏；个人partial，团队联合完整'},
 'backend/internal/handlers/util.go': {ranges:'1-140',teamReview:'full',teamRanges:[{reviewer:'audit_server_support',ranges:'1-140'},{reviewer:'audit_platform',ranges:'141-EOF'}],note:'区间并集1-EOF，无遗漏；个人partial，团队联合完整'},
};
const rows=[...files].sort().map(file=>({file,lines:fs.readFileSync(file,'utf8').split(/\r?\n/).length,review:full.has(file)?'full':partial[file]?'partial':'indexed-only',...(partial[file]||{})}));
fs.writeFileSync(path.join(root,'docs/audit-2026-11-06/server-support-coverage.json'),JSON.stringify({date:'2026-11-06',reviewer:'audit_server_support',meaning:'full=完整业务实现人工阅读（可省重复注释日志）；partial=明确区间；indexed-only=全项目结构提取，未冒称人工阅读全文',files:rows},null,2)+'\n');
console.log(Object.fromEntries(['full','partial','indexed-only'].map(k=>[k,rows.filter(r=>r.review===k).length])));
