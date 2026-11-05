# AiFi 链失联节点（华为云 ECS）控制权恢复与应急操作指引（严格零重启方案）

> **⚠️ 核心戒条**：**严禁重启服务器！严禁关机！**  
> 当前失联节点（如 `n_AiFi-Rpc1`）上的 Geth 节点进程**仍在正常出块与对外提供服务**，任何重启或关机都会造成链上服务中断。  
> 本指引专门针对**“服务器持续在线运行、业务零中断、完全不重启机器”**的在线夺回控制权方案。

---

## 一、 现状与核心原则

| 节点名称 | 公网 IP | 内网 IP (同 VPC) | 业务状态 | 目标 |
| :--- | :--- | :--- | :--- | :--- |
| **n_AiFi-Rpc1** | `124.243.174.69` | `192.168.0.178` | **Geth 正常运行中**（出块/RPC读写正常） | **在不重启、不杀进程的前提下夺回 root 权限** |
| **n_AiFi-Archive-02** | `190.92.199.31` | `192.168.0.207` | 系统在线，SSH 密钥被覆盖 | **在线恢复 SSH 密钥** |

---

## 二、 方案 A（最稳、最快）：华为云「云助手 / 执行脚本」在线注入密钥（零重启、10 秒完成）

华为云 ECS 系统内默认运行有华为云原生代理进程（`UniAgent` 与 `Cloud-init`），支持在控制台直接向运行中的机器下发 Shell 脚本，**完全无需登录、无需关机重启、业务零感知**！

### 操作步骤：
1. 登录 **华为云控制台** -> 进入 **弹性云服务器 ECS**。
2. 在左侧导航栏或实例列表中，找到 **「批量运维」** / **「云助手」** -> **「执行命令」**（或直接在 ECS 列表中勾选 `n_AiFi-Rpc1`，点击上方的 **「执行命令」**）。
3. 选择执行类型：`Shell`，执行用户：`root`。
4. 在脚本命令框中，**完整复制并粘贴以下恢复命令**：

```bash
#!/bin/bash
# 1. 确保目录与权限存在
mkdir -p /root/.ssh
chmod 700 /root/.ssh

# 2. 将正确的官方生产公钥重新写入 authorized_keys（覆盖回被破坏的文件）
cat << 'EOF' > /root/.ssh/authorized_keys
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDJnFwr928bhbJwlml6dWpb548660kf7GjKg/4mBV/DD4U0rYfa7/Nxdb1n060T8ei+CWPKjS5q67xD0ux2PRy7v3kMnz6RBv8NcnQg+NENGpfWxlyQH3/gu4dMgWfKTgdzU5xQoJxR1GgWfdfFlKe6r3n8K0XA67USevp3gxDu413miiSoLL+L3UL3xo0EHPYeRpH1CNPHQr5nHgk6upEn1fYHkuby7pBhF66TTMS47rj5ClvQJWC1Ighc4Hll34unBbe4y+kZRolAnU6k4FdvhFo6SQSPYfl5dovH8/sYmVGzAario8hktCEvx3M4O0Dguxg8zZuMZbX+/L3Q8kLvr0UDSjpFzqfSVqAcoPFBTI4BfdQzCuDmnQd9c1iXAoYgqX1mTOl1bxwmTL1rfrzftgfVPVoLKAH7ILKhn0zK1Hl2sK8RkMQW0Dvyg9Y09zyMqsEZvFLdgd+cpEcOv4gdbwdTyt92Nb5fyuz6MK/2O01q+pr6q9uokijc5/wizlU= Generated-by-Nova
EOF

# 3. 严格设置 600 权限（SSH 强制安全要求）
chmod 600 /root/.ssh/authorized_keys

# 4. 清理黑客可能写入的定时任务
rm -f /var/spool/cron/crontabs/root 2>/dev/null || true
```

5. 点击 **「执行」**。
6. **验证结果**：
   * 云助手执行结果通常在 5~10 秒内返回 `Success`（退出码 0）。
   * 本地管理员直接执行 `ssh -i KeyPair-db6d-node-prod-2.pem root@124.243.174.69`，**即可瞬间免密登录，Geth 进程全程不中断！**

---

## 三、 方案 B：华为云控制台「在线重置密码（切勿勾选重启）」+ VNC 唤醒登录

如果控制台未开启云助手脚本下发功能，可使用华为云的原生**在线免重启密码重置**功能。

### ⚠️ 特别注意事项：
* **切勿勾选“自动重启”！** 
* **切勿勾选“自动重启”！**

### 操作步骤：
1. 在 ECS 控制台找到失联节点，点击右侧 **「更多」** -> **「密码/密钥」** -> **「重置密码」**。
2. 用户名填写：`root`。
3. 输入新密码（如 `AiFi_Safe_2026!#`）。
4. **检查确认：确保不要勾选“重启”选项（华为云支持在线修改密码）**，直接点击确定提交。
5. **打开 VNC 终端**：
   * 点击 ECS 列表中的 **「远程登录」**（VNC 方式）。
   * **如果看到的是黑屏**：请先按几次回车键（Enter），或者点击顶部工具栏的 **「发送 Ctrl+Alt+Del」** 唤醒终端，屏幕即会弹出 `login:`。
   * 用户名输入：`root`
   * 密码输入：刚才在线重置的新密码。
6. **写入公钥**：登录成功后，在终端粘贴以下命令：
   ```bash
   mkdir -p /root/.ssh && chmod 700 /root/.ssh
   cat << 'EOF' > /root/.ssh/authorized_keys
   ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDJnFwr928bhbJwlml6dWpb548660kf7GjKg/4mBV/DD4U0rYfa7/Nxdb1n060T8ei+CWPKjS5q67xD0ux2PRy7v3kMnz6RBv8NcnQg+NENGpfWxlyQH3/gu4dMgWfKTgdzU5xQoJxR1GgWfdfFlKe6r3n8K0XA67USevp3gxDu413miiSoLL+L3UL3xo0EHPYeRpH1CNPHQr5nHgk6upEn1fYHkuby7pBhF66TTMS47rj5ClvQJWC1Ighc4Hll34unBbe4y+kZRolAnU6k4FdvhFo6SQSPYfl5dovH8/sYmVGzAario8hktCEvx3M4O0Dguxg8zZuMZbX+/L3Q8kLvr0UDSjpFzqfSVqAcoPFBTI4BfdQzCuDmnQd9c1iXAoYgqX1mTOl1bxwmTL1rfrzftgfVPVoLKAH7ILKhn0zK1Hl2sK8RkMQW0Dvyg9Y09zyMqsEZvFLdgd+cpEcOv4gdbwdTyt92Nb5fyuz6MK/2O01q+pr6q9uokijc5/wizlU= Generated-by-Nova
   EOF
   chmod 600 /root/.ssh/authorized_keys
   ```
7. 此时 SSH 即已恢复。

---

## 四、 华为云安全组快速止血（在控制台即可操作，秒级生效且不影响运行中进程）

在恢复 root 控制权的同时，请让 M sir 在华为云控制台检查并收紧安全组规则：
* **目标安全组**：绑在 `n_AiFi-Rpc1`、`n_AiFi-Rpc2`、`n_AiFi-Archive-01/02` 上的安全组。
* **规则检查**：
  * **入方向规则**：找到端口 `8545` 和 `8546`。
  * **修改源地址**：将来源 `0.0.0.0/0` 删除或修改，**仅允许 Nginx 内网 IP（如 `192.168.0.0/24`）访问**，严禁向公网开放 8545/8546 端口。
  * **修改安全组规则不需要重启机器，全网立即生效！**
