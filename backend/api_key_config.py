"""
API密钥配置
用于验证前端请求，防止后端被直接调用
从环境变量读取，如果环境变量不存在则使用默认值
"""

import os
from dotenv import load_dotenv

# 加载 .env 文件
load_dotenv()

# API通信密钥
# 从环境变量读取，必须与前端设置页面中的密钥保持一致
API_SECRET_KEY = os.getenv('API_SECRET_KEY', 'ovh-phantom-sniper-2024-secret-key')

# 是否启用API密钥验证
# 开发环境可以设置为 False，生产环境必须设置为 True
ENABLE_API_KEY_AUTH = os.getenv('ENABLE_API_KEY_AUTH', 'True').lower() == 'true'

# 白名单路径（不需要验证的路径）
# 例如健康检查、静态文件、内部API等
WHITELIST_PATHS = [
    '/health',
    '/api/health',
    '/api/internal/monitor/price',  # 内部监控价格API，不需要API密钥验证
    '/api/telegram/webhook',  # Telegram webhook，不需要API密钥验证（Telegram服务器调用）
]
