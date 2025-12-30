"""
API认证中间件
验证请求的API密钥，防止后端被直接调用
"""

from flask import request, jsonify
from functools import wraps
from api_key_config import API_SECRET_KEY, ENABLE_API_KEY_AUTH, WHITELIST_PATHS
import time

def require_api_key(f):
    """
    API密钥验证装饰器
    用于保护API端点，确保只有持有正确密钥的客户端才能访问
    """
    @wraps(f)
    def decorated_function(*args, **kwargs):
        # 如果未启用验证，直接放行
        if not ENABLE_API_KEY_AUTH:
            return f(*args, **kwargs)
        
        # 检查是否在白名单中
        if request.path in WHITELIST_PATHS:
            return f(*args, **kwargs)
        
        # 获取请求头中的API密钥
        api_key = request.headers.get('X-API-Key')
        
        # 验证API密钥
        if not api_key:
            return jsonify({
                'error': 'Missing API key',
                'message': '缺少API密钥，请通过官方前端访问'
            }), 401
        
        if api_key != API_SECRET_KEY:
            return jsonify({
                'error': 'Invalid API key',
                'message': 'API密钥无效'
            }), 401
        
        # 可选：验证请求时间戳（防重放攻击）
        request_time = request.headers.get('X-Request-Time')
        if request_time:
            try:
                timestamp = int(request_time)
                current_time = int(time.time() * 1000)
                time_diff = abs(current_time - timestamp)
                
                # 如果请求时间与服务器时间相差超过5分钟，拒绝请求
                if time_diff > 5 * 60 * 1000:  # 5分钟
                    return jsonify({
                        'error': 'Request expired',
                        'message': '请求已过期，请刷新页面重试'
                    }), 401
            except ValueError:
                pass  # 时间戳格式错误，忽略但继续处理
        
        return f(*args, **kwargs)
    
    return decorated_function


def init_api_auth(app):
    """
    初始化API认证
    为所有/api路由添加密钥验证
    """
    @app.before_request
    def verify_api_key():
        # 如果未启用验证，直接放行
        if not ENABLE_API_KEY_AUTH:
            return None
        
        # 放行OPTIONS请求（CORS预检请求）
        if request.method == 'OPTIONS':
            return None
        
        # 只验证/api路径
        if not request.path.startswith('/api/'):
            return None
        
        # 检查是否在白名单中
        if request.path in WHITELIST_PATHS:
            return None
        
        # 获取请求头中的API密钥
        api_key = request.headers.get('X-API-Key')
        
        # 验证API密钥
        if not api_key:
            return jsonify({
                'error': 'Missing API key',
                'message': '缺少API密钥，请通过官方前端访问',
                'code': 'NO_API_KEY'
            }), 401
        
        if api_key != API_SECRET_KEY:
            return jsonify({
                'error': 'Invalid API key',
                'message': 'API密钥无效，禁止访问',
                'code': 'INVALID_API_KEY'
            }), 401
        
        # 可选：验证请求时间戳
        request_time = request.headers.get('X-Request-Time')
        if request_time:
            try:
                timestamp = int(request_time)
                current_time = int(time.time() * 1000)
                time_diff = abs(current_time - timestamp)
                
                # 如果请求时间与服务器时间相差超过5分钟，拒绝请求
                if time_diff > 5 * 60 * 1000:
                    return jsonify({
                        'error': 'Request expired',
                        'message': '请求已过期（时间戳验证失败）',
                        'code': 'TIMESTAMP_EXPIRED'
                    }), 401
            except ValueError:
                # 时间戳格式错误，记录但不阻止请求
                pass
        
        return None


def log_api_request():
    """
    记录API请求日志（可选）
    """
    api_key = request.headers.get('X-API-Key', 'None')
    masked_key = api_key[:8] + '***' if api_key and len(api_key) > 8 else 'None'
    
    print(f"API Request: {request.method} {request.path} | Key: {masked_key}")
