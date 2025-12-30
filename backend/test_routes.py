"""测试路由注册情况"""
import sys
sys.path.insert(0, '.')
from app import app

print("=" * 60)
print("注册的路由列表：")
print("=" * 60)

routes = []
for rule in app.url_map.iter_rules():
    if '/api/ovh/account/' in rule.rule:
        routes.append({
            'endpoint': rule.endpoint,
            'methods': ','.join(sorted(rule.methods - {'HEAD', 'OPTIONS'})),
            'route': rule.rule
        })

for route in sorted(routes, key=lambda x: x['route']):
    print(f"{route['methods']:10} {route['route']}")

# 特别检查 email-history 路由
email_history_found = any('/email-history' in r['route'] for r in routes)
print("\n" + "=" * 60)
if email_history_found:
    print("✅ email-history 路由已注册")
else:
    print("❌ email-history 路由未找到！")
print("=" * 60)

