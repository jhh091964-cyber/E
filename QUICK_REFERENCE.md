# MailOps 真实功能测试 - 快速参考

## 🎯 5 分钟快速开始

### 步骤 1: 准备资源
- ✅ Cloudflare API Token
- ✅ 1 台测试服务器（Ubuntu 20.04+ / Debian 11+）
- ✅ 1 个测试域名（已在 Cloudflare）

### 步骤 2: 创建配置文件
```bash
cd /workspace/cli
nano my_test_servers.csv
```

填入以下内容（替换为你的真实信息）：
```csv
row_id,cf_api_token,cf_zone,server_ip,server_port,server_user,server_password,host,domain,deploy_profile,email_use,solution
1,YOUR_API_TOKEN_HERE,example.com,YOUR_SERVER_IP,22,YOUR_USER,YOUR_PASSWORD,,mail,mail1.example.com,postfix_dovecot,test,测试
```

### 步骤 3: 运行干测试
```bash
bash test_setup.sh
# 选择选项 1 (干运行测试)
```

### 步骤 4: 真实部署
```bash
bash test_setup.sh
# 选择选项 2 (真实部署)
```

### 步骤 5: 验证结果
```bash
# 检查邮件服务
ssh root@YOUR_SERVER_IP "systemctl status postfix dovecot"

# 检查端口
ssh root@YOUR_SERVER_IP "netstat -tlnp | grep -E ':(25|587|465|143|993)'"

# 检查 DNS (在本地)
dig mail1.example.com
```

---

## 📋 常用命令

### 手动运行 CLI
```bash
# 干运行
echo '{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":1,"dry_run":true}' | ./mailops --event-stream

# 真实部署
echo '{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":1,"dry_run":false}' | ./mailops --event-stream

# 并发部署（3台服务器）
echo '{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":3,"dry_run":false}' | ./mailops --event-stream
```

### 停止部署
```bash
# 按 Ctrl+C 停止 CLI
```

### 查看日志
```bash
# 查看最新日志
ls -lt output/logs/ | head -5

# 查看特定日志
tail -f output/logs/test_20260201_123456.log
```

---

## 🔧 配置字段说明

| 字段 | 说明 | 示例 | 必填 |
|------|------|------|------|
| row_id | 行号 | 1 | ✅ |
| cf_api_token | Cloudflare API Token | `abc123...xyz789` | ✅ |
| cf_zone | 域名 | `example.com` | ✅ |
| server_ip | 服务器 IP | `1.2.3.4` | ✅ |
| server_port | SSH 端口 | 22 | ✅ |
| server_user | SSH 用户名 | `root` | ✅ |
| server_password | SSH 密码 | `MyPassword123` | ✅ |
| server_key_path | SSH 密钥路径 | `/root/.ssh/id_rsa` | ❌ |
| host | 邮件主机名 | `mail` | ✅ |
| domain | 完整域名 | `mail1.example.com` | ✅ |
| deploy_profile | 部署方式 | `postfix_dovecot` | ✅ |
| email_use | 用途 | `transactional` | ✅ |
| solution | 方案名称 | `测试案例1` | ✅ |

### deploy_profile 选项
- `postfix_dovecot` - 传统方式，直接安装到系统
- `docker_mailserver` - Docker 容器方式

### email_use 选项
- `transactional` - 事务邮件
- `internal` - 内部邮件
- `test` - 测试用途

---

## ⚠️ 常见错误速查

| 错误代码 | 错误类型 | 解决方法 |
|---------|---------|---------|
| `SSH_CONN` | SSH 连接失败 | 检查 IP、端口、用户名、密码 |
| `SSH_TIMEOUT` | SSH 超时 | 检查网络连接，增加超时时间 |
| `DNS_AUTH_FAILED` | Cloudflare 认证失败 | 检查 API Token 权限 |
| `DNS_RATE_LIMIT` | Cloudflare 速率限制 | 等待几分钟后重试 |
| `DEPLOY_FAILED` | 部署失败 | 查看详细日志，检查服务器配置 |
| `AUTH_FAILED` | 认证失败 | 检查 SSH 凭据 |

---

## 🔍 验证清单

### 服务器端验证
```bash
# SSH 连接测试
ssh -p 22 root@YOUR_SERVER_IP

# 检查邮件服务状态
systemctl status postfix
systemctl status dovecot

# 检查端口监听
netstat -tlnp | grep -E ':(25|587|465|143|993)'

# 检查 DKIM 密钥
cat /etc/opendkim/keys/example.com/mail.private

# 检查日志
journalctl -u postfix -n 50
journalctl -u dovecot -n 50
tail -f /var/log/mail.log
```

### DNS 验证
```bash
# 检查 A 记录
dig mail1.example.com

# 检查 MX 记录
dig mx example.com

# 检查 TXT 记录 (SPF)
dig txt example.com

# 检查 DMARC
dig _dmarc.example.com txt

# 检查 DKIM
dig default._domainkey.mail1.example.com txt
```

### Cloudflare Dashboard 验证
1. 登录 Cloudflare
2. 选择域名 → DNS → Records
3. 确认以下记录已创建：
   - A: `mail` → 服务器 IP
   - MX: `@` → `mail.example.com` (优先级 10)
   - TXT: SPF 记录
   - TXT: DMARC 记录
   - TXT: DKIM 记录

---

## 📊 性能测试

### 单服务器部署
```bash
time echo '{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":1,"dry_run":false}' | ./mailops --event-stream
```

### 并发部署（5 台）
```bash
time echo '{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":5,"dry_run":false}' | ./mailops --event-stream
```

### 压力测试（10 台）
```bash
time echo '{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":10,"dry_run":false}' | ./mailops --event-stream
```

---

## 📞 获取帮助

### 查看完整文档
```bash
cat REAL_TESTING_GUIDE.md
```

### 查看 CLI 帮助
```bash
./mailops --help
```

### GUI 测试
```bash
# GUI 模拟测试地址
https://000bl.app.super.myninja.ai/test/test.html
```

---

## 🎓 测试场景

### 场景 1: 首次部署测试
1. 使用 1 台服务器
2. 使用干运行模式验证配置
3. 执行真实部署
4. 验证所有服务正常运行

### 场景 2: 多服务器并发测试
1. 准备 3 台服务器
2. 并发部署所有服务器
3. 验证部署顺序和资源使用
4. 检查所有服务器状态

### 场景 3: 错误恢复测试
1. 使用错误的密码触发错误
2. 观察重试机制
3. 验证错误报告准确性

### 场景 4: 混合部署测试
1. 部署 postfix_dovecot 到服务器 A
2. 部署 docker_mailserver 到服务器 B
3. 对比两种方式的差异

---

## ✅ 成功标志

测试成功的标志：
- ✅ CLI 无错误退出
- ✅ 所有任务状态为 SUCCESS
- ✅ 邮件服务正在运行
- ✅ 端口正常监听
- ✅ DNS 记录已创建
- ✅ 可以发送测试邮件

---

**📌 记住：先干运行，后真实部署！**