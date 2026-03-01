"""
@author        : ZhangPeiCheng
@function      : Qwen Chat Server (Aliyun DashScope) - Drop-in replacement for DeepSeek
@time          : 2026/02/20
"""
#!/usr/bin/env python3
from flask import Flask, request, jsonify
from openai import OpenAI
import logging
import time
import hashlib
from collections import OrderedDict

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# === 阿里云 Qwen 配置 ===
MODEL_NAME = "qwen-plus"
API_KEY = "sk-2b3e4f939b734798977081f73b83ad2f"  # 写死（不上传仓库）
BASE_URL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

# 初始化 OpenAI 兼容客户端
client = OpenAI(
    api_key=API_KEY,
    base_url=BASE_URL
)

# === 请求去重缓存（允许最多 10 次重放）===
class DedupCache:
    def __init__(self, max_size=1000, ttl=60, max_retries=10):
        self.cache = OrderedDict()  # key -> (count, first_seen_time)
        self.max_size = max_size
        self.ttl = ttl
        self.max_retries = max_retries

    def _clean_expired(self):
        now = time.time()
        expired_keys = [
            k for k, (_, first_time) in self.cache.items()
            if now - first_time > self.ttl
        ]
        for k in expired_keys:
            del self.cache[k]

    def should_block(self, key: str) -> bool:
        """返回是否应拦截请求（第 11 次及以后）"""
        self._clean_expired()
        now = time.time()

        if key in self.cache:
            count, first_time = self.cache[key]
            new_count = count + 1
            self.cache[key] = (new_count, first_time)
            return new_count > self.max_retries
        else:
            self.cache[key] = (1, now)
            if len(self.cache) > self.max_size:
                self.cache.popitem(last=False)  # FIFO
            return False

dedup_cache = DedupCache(max_retries=10)

def generate_fingerprint(data: dict) -> str:
    messages = data.get('messages', [])
    temp = data.get('temperature', 0.7)
    max_toks = data.get('max_tokens', 512)
    stream = data.get('stream', False)
    msg_str = "|".join([
        f"{m.get('role', '')}:{m.get('content', '')[:100]}"
        for m in messages
    ])
    raw = f"{msg_str}|{temp}|{max_toks}|{stream}"
    return hashlib.md5(raw.encode('utf-8')).hexdigest()


@app.route('/health', methods=['GET'])
def health_check():
    return jsonify({
        'status': 'healthy',
        'model': MODEL_NAME,
        'timestamp': time.time(),
        'provider': 'aliyun-dashscope'
    })


@app.route('/chat/completions', methods=['POST'])
def chat_completion():
    try:
        data = request.get_json()
        if not data or 'messages' not in data:
            return jsonify({'error': 'messages field is required'}), 400

        # 重放容忍：前 10 次放行，第 11 次拦截
        fingerprint = generate_fingerprint(data)
        if dedup_cache.should_block(fingerprint):
            logger.warning("🔁 Blocked after %d retries (fingerprint: %s)",
                           dedup_cache.max_retries + 1, fingerprint[:8])
            return jsonify({'error': 'too many duplicate requests'}), 429

        messages = data['messages']
        stream = data.get('stream', False)
        temperature = min(data.get('temperature', 0.7), 1.0)
        max_tokens = min(data.get('max_tokens', 4096), 8192)

        logger.info(f"→ Calling Qwen ({MODEL_NAME}) with {len(messages)} messages")

        response = client.chat.completions.create(
            model=MODEL_NAME,
            messages=messages,
            stream=stream,
            temperature=temperature,
            max_tokens=max_tokens,
            timeout=300,
        )

        if stream:
            def generate():
                for chunk in response:
                    delta = chunk.choices[0].delta
                    if hasattr(delta, 'content') and delta.content:
                        yield delta.content
            return app.response_class(generate(), mimetype='text/plain')
        else:
            usage = getattr(response, 'usage', None)
            return jsonify({
                'choices': [{
                    'message': {
                        'role': response.choices[0].message.role,
                        'content': response.choices[0].message.content
                    },
                    'finish_reason': response.choices[0].finish_reason
                }],
                'usage': {
                    'prompt_tokens': getattr(usage, 'prompt_tokens', 0),
                    'completion_tokens': getattr(usage, 'completion_tokens', 0),
                    'total_tokens': getattr(usage, 'total_tokens', 0)
                } if usage else None,
                'model': MODEL_NAME,
                'timestamp': time.time()
            })

    except Exception as e:
        logger.error(f"❌ Qwen API error: {str(e)}")
        return jsonify({'error': str(e)}), 500


@app.route('/quick-chat', methods=['POST'])
def quick_chat():
    try:
        data = request.get_json()
        message = data.get('message', '').strip()
        if not message:
            return jsonify({'error': 'message is required'}), 400

        # 重放容忍
        fingerprint = generate_fingerprint(data)
        if dedup_cache.should_block(fingerprint):
            logger.warning("🔁 Quick-chat blocked after too many retries (fp: %s)", fingerprint[:8])
            return jsonify({'error': 'too many duplicate requests'}), 429

        system_prompt = data.get('system_prompt', 'You are a helpful assistant.')
        messages = [
            {"role": "system", "content": system_prompt[:500]},
            {"role": "user", "content": message[:1000]}
        ]

        logger.info(f"→ Quick chat: {message[:50]}...")

        response = client.chat.completions.create(
            model=MODEL_NAME,
            messages=messages,
            temperature=0.7,
            max_tokens=256
        )

        content = response.choices[0].message.content
        usage = getattr(response, 'usage', None)

        return jsonify({
            'response': content,
            'usage': {
                'prompt_tokens': getattr(usage, 'prompt_tokens', 0),
                'completion_tokens': getattr(usage, 'completion_tokens', 0),
                'total_tokens': getattr(usage, 'total_tokens', 0)
            } if usage else None,
            'timestamp': time.time()
        })

    except Exception as e:
        logger.error(f"❌ Quick chat error: {str(e)}")
        return jsonify({'error': str(e)}), 500


if __name__ == '__main__':
    logger.info("Starting Qwen Chat Server with Replay Tolerance (max 10)")
    logger.info(f"   Model: {MODEL_NAME}")
    logger.info(f"   Base URL: {BASE_URL}")
    app.config['TIMEOUT'] = 300
    app.run(host='0.0.0.0', port=20001, threaded=True)