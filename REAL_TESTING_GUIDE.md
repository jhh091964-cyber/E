# MailOps 真实功能测试操作指南

## 🎯 测试目标
验证 MailOps 系统的真实部署功能，包括：
- SSH 连接和远程命令执行
- 邮件服务器软件安装（Postfix + Dovecot 或 Docker MailServer）
- DKIM 密钥生成
- Cloudflare DNS 记录自动配置
- 健康检查和端口验证
- 并发部署能力

---

## 📋 测试前准备清单

### 1. Cloudflare 准备
- [ ] 拥有 Cloudflare 账户
- [ ] 至少 1 个域名已在 Cloudflare 托管
- [ ] 创建 API Token（步骤见下文）

#### 创建 Cloudflare API Token
1. 登录 Cloudflare Dashboard
2. 点击右上角头像 → **My Profile**
3. 选择左侧 **API Tokens**
4. 点击 **Create Token**
5. 使用模板 **Edit zone DNS**
6. 配置权限：
   - **Zone** → **DNS** → **Edit**
   - **Zone Resources** → **Include** → **Specific zone** → 选择你的域名
7. 设置过期时间（建议测试用 1 天）
8. 点击 **Continue to summary** → **Create Token**
9. **复制生成的 Token**（只显示一次！）

### 2. 服务器准备
- [ ] 1 台及以上测试服务器
- [ ] 操作系统：Ubuntu 20.04+ 或 Debian 11+
- [ ] SSH 访问权限
- [ ] sudo 权限（需要安装软件包）

#### 服务器要求
- **最低配置**: 1 CPU, 1GB RAM, 20GB 磁盘
- **网络**: 开放端口 25, 587, 465, 143, 993
- **防火墙**: 允许 SSH (22) 和邮件端口
- **权限**: 能够使用 `sudo` 安装软件包

### 3. 域名准备
- [ ] 准备测试子域名，例如：
  - `mail1.example.com`
  - `mail2.example.com`
- [ ] 确保域名 DNS 已托管在 Cloudflare
- [ ] 这些域名将用于：
  - A 记录: `mail` → 服务器 IP
  - MX 记录: `example.com` → `mail.example.com`
  - TXT 记录: SPF, DMARC, DKIM

---

## 📝 填写配置文件

### 配置文件格式：`test_servers.csv`

```csv
row_id,cf_api_token,cf_zone,server_ip,server_port,server_user,server_password,server_key_path,host,domain,deploy_profile,email_use,solution
```

### 字段说明

| 字段 | 说明 | 示例 | 必填 |
|------|------|------|------|
| row_id | 行号 | 1 | ✅ |
| cf_api_token | Cloudflare API Token | `abc123...` | ✅ |
| cf_zone | Cloudflare Zone（域名） | `example.com` | ✅ |
| server_ip | 服务器 IP 地址 | `1.2.3.4` | ✅ |
| server_port | SSH 端口 | 22 | ✅ |
| server_user | SSH 用户名 | `root` | ✅ |
| server_password | SSH 密码 | `MyPassword123` | ✅ |
| server_key_path | SSH 密钥路径（可选） | `/root/.ssh/id_rsa` | ❌ |
| host | 邮件服务器主机名 | `mail` | ✅ |
| domain | 完整域名 | `mail1.example.com` | ✅ |
| deploy_profile | 部署配置文件 | `postfix_dovecot` 或 `docker_mailserver` | ✅ |
| email_use | 邮件用途 | `transactional`, `internal`, `test` | ✅ |
| solution | 解决方案名称 | `测试案例1` | ✅ |

### 配置示例

#### 示例 1: 使用密码认证 + Postfix + Dovecot
```csv
1,abc123def456,example.com,1.2.3.4,22,root,MyPassword123,,mail,mail1.example.com,postfix_dovecot,transactional,测试案例1
```

#### 示例 2: 使用密钥认证 + Docker MailServer
```csv
2,xyz789abc,example.com,5.6.7.8,22,ubuntu,,/home/ubuntu/.ssh/id_rsa,mailserver,mail2.example.com,docker_mailserver,internal,测试案例2
```

#### 示例 3: 并发测试（3 台服务器）
```csv
1,token123,example.com,1.2.3.4,22,root,password1,,mail,mail1.example.com,postfix_dovecot,test,服务器A
2,token123,example.com,5.6.7.8,22,root,password2,,mail,mail2.example.com,postfix_dovecot,test,服务器B
3,token123,example.com,9.10.11.12,22,root,password3,,mail,mail3.example.com,postfix_dovecot,test,服务器C
```

---

## 🚀 测试步骤

### 步骤 1: 准备配置文件

1. 复制模板文件：
   ```bash
   cp /workspace/cli/test_servers.csv /workspace/cli/my_test_servers.csv
   ```

2. 编辑配置文件，填入真实信息：
   ```bash
   nano /workspace/cli/my_test_servers.csv
   ```

3. 保存文件

### 步骤 2: 干运行测试（推荐先执行）

使用 `--dns-dry-run` 参数测试，不实际创建 DNS 记录：

```bash
cd /workspace/cli
./mailops --event-stream --config my_test_servers.csv --dns-dry-run --concurrency 1
```

**通过标准输入发送 START_RUN 命令**：
```bash
echo '{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":1,"dry_run":false}' | ./mailops --event-stream
```

### 步骤 3: 真实部署

确认干运行成功后，进行真实部署：

```bash
echo '{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":2,"dry_run":false}' | ./mailops --event-stream
```

### 步骤 4: 监控输出

CLI 会输出 NDJSON 格式的事件流，包括：
- `RUN_STARTED`: 任务开始
- `TASK_STATE`: 任务状态变化
- `TASK_STEP`: 部署步骤进度
- `LOG_LINE`: 详细日志
- `RUN_PROGRESS`: 进度统计
- `RUN_FINISHED`: 任务完成

---

## 📊 验证测试结果

### 1. 检查 SSH 连接
```bash
ssh -p 22 root@YOUR_SERVER_IP
```

### 2. 检查邮件服务状态
```bash
# Postfix
systemctl status postfix

# Dovecot
systemctl status dovecot

# Docker (如果使用 docker_mailserver)
docker ps
```

### 3. 检查端口监听
```bash
netstat -tlnp | grep -E ':(25|587|465|143|993)\s'
```

### 4. 检查 Cloudflare DNS
登录 Cloudflare Dashboard，查看：
- **DNS Records** → A 记录是否创建
- **DNS Records** → MX 记录是否创建
- **DNS Records** → TXT 记录（SPF, DMARC, DKIM）是否创建

### 5. 检查 DKIM 密钥
```bash
# Postfix + Dovecot
cat /etc/opendkim/keys/example.com/mail.private

# Docker MailServer
docker exec mailserver cat /etc/opendkim/keys/example.com/mail.private
```

### 6. 测试邮件发送
```bash
# 测试发送邮件
echo "Test email body" | mail -s "Test Subject" test@example.com
```

---

## 🔍 常见问题排查

### 问题 1: SSH 连接失败
**错误信息**: `SSH_CONN` 或 `SSH_TIMEOUT`

**解决方法**:
1. 检查服务器 IP 和端口是否正确
2. 确认 SSH 服务正在运行
3. 检查防火墙是否允许 SSH 连接
4. 验证用户名和密码/密钥

### 问题 2: Cloudflare API 错误
**错误信息**: `DNS_AUTH_FAILED` 或 `DNS_RATE_LIMIT`

**解决方法**:
1. 验证 API Token 是否正确
2. 检查 Token 权限是否包含 DNS Edit
3. 确认 Token 作用域包含正确的域名
4. 等待速率限制重置（Cloudflare 限制）

### 问题 3: 软件包安装失败
**错误信息**: `DEPLOY_FAILED`

**解决方法**:
1. 检查服务器网络连接
2. 确认软件包源配置正确
3. 检查磁盘空间是否足够
4. 查看详细日志了解具体错误

### 问题 4: DNS 记录未生效
**解决方法**:
1. 等待 DNS 传播（通常 1-5 分钟）
2. 使用 `nslookup` 或 `dig` 命令验证 DNS 记录
3. 检查 Cloudflare DNS 页面确认记录已创建

---

## ⚠️ 安全注意事项

1. **保护敏感信息**:
   - 不要将包含真实密码的 CSV 文件提交到版本控制
   - 使用后删除或加密测试配置文件
   - 定期更换 Cloudflare API Token

2. **测试环境隔离**:
   - 使用专用测试服务器
   - 使用测试域名
   - 不要在生产环境首次部署

3. **访问控制**:
   - 测试完成后关闭不必要的端口
   - 删除测试账户
   - 清理测试数据

---

## 📈 测试报告模板

测试完成后，请记录以下信息：

### 测试环境
- 服务器数量: ____ 台
- 服务器配置: ____
- 操作系统: ____
- 网络带宽: ____

### 测试配置
- 并发数: ____
- 部署配置: ____ (postfix_dovecot / docker_mailserver)
- 测试域名: ____

### 测试结果
- 成功数量: ____ / ____
- 失败数量: ____ / ____
- 平均部署时间: ____ 分钟
- 最长部署时间: ____ 分钟
- 最短部署时间: ____ 分钟

### 遇到的问题
1. ____ 
2. ____

### 改进建议
1. ____
2. ____

---

## 🎓 进阶测试

### 1. 并发性能测试
测试不同并发数的性能：
```bash
# 并发 1
echo '{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":1,"dry_run":false}' | ./mailops --event-stream

# 并发 5
echo '{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":5,"dry_run":false}' | ./mailops --event-stream

# 并发 10
echo '{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":10,"dry_run":false}' | ./mailops --event-stream
```

### 2. 重试机制测试
故意制造错误测试重试功能：
- 使用错误的密码触发重试
- 临时关闭服务器测试连接重试

### 3. 取消功能测试
在部署过程中发送取消命令：
```bash
# 启动部署
./mailops --event-stream

# 在另一个终端发送取消命令
echo '{"type":"CANCEL_RUN"}' | nc localhost 8080
```

### 4. 混合部署测试
同时使用不同的部署配置文件：
```csv
1,token123,example.com,1.2.3.4,22,root,password1,,mail,mail1.example.com,postfix_dovecot,test,传统部署
2,token123,example.com,5.6.7.8,22,root,password2,,mailserver,mail2.example.com,docker_mailserver,test,Docker部署
```

---

## 📞 获取帮助

如果遇到问题：
1. 查看详细日志输出
2. 检查服务器日志: `journalctl -u postfix`, `journalctl -u dovecot`
3. 查看系统日志: `/var/log/syslog`, `/var/log/mail.log`
4. 联系技术支持

---

## ✅ 测试完成检查清单

- [ ] 所有服务器成功部署邮件服务
- [ ] Cloudflare DNS 记录全部创建
- [ ] 邮件服务端口正常监听
- [ ] DKIM 密钥成功生成
- [ ] SPF/DMARC 记录配置正确
- [ ] 可以成功发送测试邮件
- [ ] 并发部署功能正常
- [ ] 错误处理和重试机制工作正常
- [ ] 日志记录完整准确
- [ ] 测试报告已生成

---

**祝您测试顺利！**