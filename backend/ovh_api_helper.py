"""
OVH API Helper - 提供重试机制和限流功能
解决 SSL 连接错误和 API 限流问题
"""

import time
import logging
from threading import Lock
from tenacity import retry, stop_after_attempt, wait_exponential, retry_if_exception_type

try:
    from requests.exceptions import SSLError, ConnectionError, Timeout
except ImportError:
    # 如果 requests 未安装，定义基础异常类
    class SSLError(Exception):
        pass
    class ConnectionError(Exception):
        pass
    class Timeout(Exception):
        pass


class APIRateLimiter:
    """API 请求限流器，防止触发速率限制"""
    
    def __init__(self, max_calls_per_second=10):
        """
        初始化限流器
        
        Args:
            max_calls_per_second: 每秒最大请求数，默认 10
        """
        self.max_calls_per_second = max_calls_per_second
        self.min_interval = 1.0 / max_calls_per_second
        self.last_call_time = 0
        self.lock = Lock()
        self.logger = logging.getLogger(__name__)
    
    def wait_if_needed(self):
        """如果需要，等待以满足速率限制"""
        with self.lock:
            now = time.time()
            elapsed = now - self.last_call_time
            
            if elapsed < self.min_interval:
                sleep_time = self.min_interval - elapsed
                self.logger.debug(f"速率限制：等待 {sleep_time:.2f} 秒")
                time.sleep(sleep_time)
            
            self.last_call_time = time.time()


class OVHAPIHelper:
    """OVH API 辅助类，提供重试和限流功能"""
    
    def __init__(self, client, max_calls_per_second=10, max_retries=3):
        """
        初始化 API 辅助类
        
        Args:
            client: OVH client 实例
            max_calls_per_second: 每秒最大请求数
            max_retries: 最大重试次数
        """
        self.client = client
        self.rate_limiter = APIRateLimiter(max_calls_per_second)
        self.max_retries = max_retries
        self.logger = logging.getLogger(__name__)
        
        # 错误统计
        self.ssl_error_count = 0
        self.total_requests = 0
        self.failed_requests = 0
    
    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=2, max=10),
        retry=retry_if_exception_type((SSLError, ConnectionError, Timeout, OSError)),
    )
    def _call_with_retry(self, method, path, **params):
        """
        带重试机制的 API 调用
        
        Args:
            method: HTTP 方法 (GET, POST, PUT, DELETE)
            path: API 路径
            **params: 请求参数
            
        Returns:
            API 响应
        """
        # 限流
        self.rate_limiter.wait_if_needed()
        self.total_requests += 1
        
        try:
            method = method.upper()
            
            if method == 'GET':
                result = self.client.get(path, **params)
            elif method == 'POST':
                result = self.client.post(path, **params)
            elif method == 'PUT':
                result = self.client.put(path, **params)
            elif method == 'DELETE':
                result = self.client.delete(path, **params)
            else:
                raise ValueError(f"不支持的 HTTP 方法: {method}")
            
            # 重置 SSL 错误计数
            self.ssl_error_count = 0
            return result
            
        except SSLError as e:
            self.ssl_error_count += 1
            self.failed_requests += 1
            self.logger.warning(f"SSL 错误 (#{self.ssl_error_count}): {str(e)}")
            
            if self.ssl_error_count >= 5:
                self.logger.error("⚠️ 连续 SSL 错误过多！请检查：")
                self.logger.error("  1. 网络连接是否正常")
                self.logger.error("  2. 防火墙/代理设置")
                self.logger.error("  3. 系统时间是否准确")
                self.logger.error("  4. SSL 证书是否过期")
            
            raise
            
        except (ConnectionError, Timeout) as e:
            self.failed_requests += 1
            self.logger.warning(f"网络错误: {str(e)}")
            raise
            
        except Exception as e:
            self.failed_requests += 1
            self.logger.error(f"API 调用失败: {str(e)}")
            raise
    
    def get(self, path, **params):
        """GET 请求"""
        return self._call_with_retry('GET', path, **params)
    
    def post(self, path, **params):
        """POST 请求"""
        return self._call_with_retry('POST', path, **params)
    
    def put(self, path, **params):
        """PUT 请求"""
        return self._call_with_retry('PUT', path, **params)
    
    def delete(self, path, **params):
        """DELETE 请求"""
        return self._call_with_retry('DELETE', path, **params)
    
    def get_stats(self):
        """获取统计信息"""
        success_rate = 0
        if self.total_requests > 0:
            success_rate = ((self.total_requests - self.failed_requests) / self.total_requests) * 100
        
        return {
            'total_requests': self.total_requests,
            'failed_requests': self.failed_requests,
            'success_rate': f'{success_rate:.1f}%',
            'ssl_error_count': self.ssl_error_count
        }


# 全局实例（可选）
_global_helper = None

def get_global_helper(client, max_calls_per_second=10):
    """获取全局 API 辅助实例"""
    global _global_helper
    if _global_helper is None or _global_helper.client != client:
        _global_helper = OVHAPIHelper(client, max_calls_per_second)
    return _global_helper
