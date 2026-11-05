# AiFi 链失联节点（华为云 ECS）控制权恢复与应急操作指引

> **适用对象**：运维管理员（M sir）及安全应急小组  
> **文档版本**：v1.0  
> **更新时间**：2026-11-05  
> **紧急程度**：P0 级应急操作  

---

## 一、 失联目标节点与现状说明

| 节点名称 | 公网 IP | 内网 IP (同 VPC) | 当前状态 | 现状诊断 |
| :--- | :--- | :--- | :--- | :--- |
| **n_AiFi-Rpc1** | `124.243.174.69` | `192.168.0.178` | SSH 报 `Permission denied` | Geth 进程仍在运行，但 `/root/.ssh/authorized_keys` 被恶意覆盖破坏 |
| **n_AiFi-Archive-02** | `190.92.199.31` | `192.168.0.207` | SSH 报 `Permission denied` | Geth 进程已退，`/root/.ssh/authorized_keys` 被破坏 |

> **同环境存活可用辅助节点**：
> * **n_AiFi-Rpc2**：公网 `188.239.46.173` / 内网 `192.168.0.126`（运行正常，可用于挂载系统盘救援）

---

## 二、 为什么 VNC“访问不了”？（现象排查）

请根据 M sir 在华为云控制台打开 VNC 时的具体表现进行对应：

1. **表现 1：VNC 窗口纯黑屏，无任何反应**
   * **原因**：Linux 终端进入休眠/屏保状态。
   * **解决**：在 VNC 窗口中连续按 **Enter（回车）** 键数次，或者点击 VNC 顶部控制栏的 **「发送 Ctrl+Alt+Del」**，屏幕即可唤醒并弹出 `login:` 登录提示。
2. **表现 2：出现 `login:` 提示，但输入任何密码均提示 `Login incorrect`**
   * **核心原因**：机器创建之初采用的是 **SSH 密钥对（KeyPair）** 方式，Linux 默认**未给 root 设置过密码**（root 在 `/etc/shadow` 中被锁定为 `!` 或 `*`）。
   * **为什么以前能登现在不能**：黑客此前利用 Geth RPC 的 `debug_writeMemProfile` 漏洞覆盖了 `/root/.ssh/authorized_keys`，导致原 SSH 密钥失效；而 VNC 必须走密码验证，因无密码故无法登入。
3. **表现 3：浏览器无法连接 VNC（Websocket 握手失败/连接超时）**
   * **解决**：请尝试更换 Chrome 浏览器无痕窗口，或重新登录华为云控制台刷新 Token。

---

## 三、 夺回 root 控制权的 3 种实操方案（按推荐优先级）

---

### 🌟 方案一：华为云控制台「一键重置密码 / 注入密钥」（推荐，耗时 2 分钟，零破坏）

华为云 ECS 系统内运行有华为云原生代理（`UniAgent` 与 `Cloud-init`），支持从云控制台底层强制重置 root 密码。

#### 操作步骤：
1. 打开 **华为云控制台** -> 进入 **弹性云服务器 ECS** 列表。
2. 找到目标机器（以 `n_AiFi-Rpc1` 为例）：
   * 点击操作列的 **「更多」** -> **「密码/密钥」** -> **「重置密码」**。
3. 在重置弹窗中：
   * 用户名：`root`
   * 输入并记住新密码（如：`AiFi_Sec_2026!#`）。
   * 若提示“在线重置”，直接点击确定；若提示需重启，勾选“自动重启”并确定。
4. **通过 VNC 登入**：
   * 刷新控制台后，再次打开该机器的 **VNC 登录**。
   * 账号输入 `root`，密码输入刚刚设置的新密码，即可成功登录！
5. **恢复 SSH 权限（登录后在 VNC 终端执行）**：
   ```bash
   # 1. 确保 .ssh 目录存在并权限正确
   mkdir -p /root/.ssh && chmod 700 /root/.ssh

   # 2. 将正确的官方生产公钥重新写入 authorized_keys
   cat << 'EOF' > /root/.ssh/authorized_keys
   ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDJnFwr928bhbJwlml6dWpb548660kf7GjKg/4mBV/DD4U0rYfa7/Nxdb1n060T8ei+CWPKjS5q67xD0ux2PRy7v3kMnz6RBv8NcnQg+NENGpfWxlyQH3/gu4dMgWfKTgdzU5xQoJxR1GgWfdfFlKe6r3n8K0XA67USevp3gxDu413miiSoLL+L3UL3xo0EHPYeRpH1CNPHQr5nHgk6upEn1fYHkuby7pBhF66TTMS47rj5ClvQJWC1Ighc4Hll34unBbe4y+kZRolAnU6k4FdvhFo6SQSPYfl5dovH8/sYmVGzAario8hktCEvx3M4O0Dguxg8zZuMZbX+/L3Q8kLvr0UDSjpFzqfSVqAcoPFBTI4BfdQzCuDmnQd9c1iXAoYgqX1mTOl1bxwmTL1rfrzftgfVPVoLKAH7ILKhn0zK1Hl2sK8RkMQW0Dvyg9Y09zyMqsEZvFLdgd+cpEcOv4gdbwdTyt92Nb5fyuz6MK/2O01q+pr6q9uokijc5/wizlU= Generated-by-Nova
   EOF

   # 3. 设置权限
   chmod 600 /root/.ssh/authorized_keys
   ```
6. **验证**：本地管理员通过终端执行 `ssh -i KeyPair-db6d-node-prod-2.pem root@<公网IP>` 即可无密码正常登录。

---

### 🛡️ 方案二：控制台打快照 + 系统盘挂载至 Rpc2 救援（100% 成功，数据零风险）

如果方案一重置密码后仍无法登入（说明系统的 PAM 认证或 `/etc/shadow` 遭到损坏），采用云硬盘挂载救援是**最稳妥、绝不丢失区块数据**的专业方案。

> **关键说明**：区块链的数据都在独立的数据盘（`/dev/vdb` 或挂载于 `/data`），本次操作**仅针对 40GB 的操作系统盘**，绝对不会触碰区块链账本数据。

#### 操作步骤：

#### 第 1 步：打快照兜底（保底保障）
* 在华为云控制台 -> **云硬盘 EVS**，找到故障机器的系统盘与数据盘，分别点击 **「创建快照」**（防范任何误操作）。

#### 第 2 步：关机并卸载系统盘
1. 在 ECS 控制台将故障机（如 `n_AiFi-Rpc1`）正常关机（已关机状态）。
2. 点击进入该实例详情 -> **「云硬盘」** 选项卡。
3. 找到系统盘（设备名为 `/dev/vda`，大小约 40GB），点击 **「卸载」**。此时系统盘变为“可用”状态。

#### 第 3 步：挂载至存活机器 `n_AiFi-Rpc2`
1. 找到刚才卸载的系统盘，点击 **「挂载」**。
2. 目标服务器选择同 VPC 下正常运行的 **`n_AiFi-Rpc2`**（公网 `188.239.46.173`），挂载为数据盘。

#### 第 4 步：登录 `Rpc2` 进行文件修复
SSH 登录正常的 `n_AiFi-Rpc2`，执行如下命令：

```bash
# 1. 确认新盘的设备号（通常为 /dev/vdc 或 /dev/vdb）
lsblk

# 2. 挂载故障机的根分区到 /mnt（注意：根据 lsblk 选择实际的 40G 盘的分区，如 /dev/vdc1）
mkdir -p /mnt/rescue
mount /dev/vdc1 /mnt/rescue

# 3. 直接将 Rpc2 正常的 SSH 密钥拷入故障盘，瞬间夺回控制权！
cp -p /root/.ssh/authorized_keys /mnt/rescue/root/.ssh/authorized_keys
chmod 700 /mnt/rescue/root/.ssh
chmod 600 /mnt/rescue/root/.ssh/authorized_keys

# 4. 检查故障机关键认证文件是否被覆盖
# 正常情况下第一行应为 root:x:0:0:root:/root:/bin/bash
head -n 2 /mnt/rescue/etc/passwd

# 如果 passwd / shadow 出现乱码（被 pprof 覆盖），直接从 Rpc2 同步一份：
# cp -p /etc/passwd /mnt/rescue/etc/passwd
# cp -p /etc/shadow /mnt/rescue/etc/shadow

# 5. 清理潜在的黑客恶意 cron 任务
ls -la /mnt/rescue/var/spool/cron/crontabs/
rm -f /mnt/rescue/var/spool/cron/crontabs/root

# 6. 安全卸载
umount /mnt/rescue
```

#### 第 5 步：挂回原机器并开机
1. 回到华为云控制台，在 `Rpc2` 的云硬盘中将该盘 **「卸载」**。
2. 重新找到故障机（`Rpc1`），点击 **「挂载磁盘」**，选择刚才修复完的系统盘，**挂载点选择系统启动盘（/dev/vda）**。
3. 启动机器 `Rpc1`。
4. 本地终端测试 SSH 登录，立即恢复正常！

---

### ⚡ 方案三：VNC 单用户模式（GRUB `init=/bin/bash`）

如果 M sir 具备 Linux 底层引导运维习惯，亦可直接在 VNC 里单用户破解：

1. 在华为云控制台点击机器 **「重启」**。
2. 迅速在控制台打开 **VNC 登录窗口**，在黑底蓝字的 GRUB 菜单出现时，按方向键停止倒计时。
3. 光标停留在默认内核行，按键盘字母 **`e`** 进入编辑。
4. 找到以 `linux` 或 `linux16` 开头的那一行，移动光标到该行末尾，输入空格并追加：
   ```text
   init=/bin/bash
   ```
5. 按 **`Ctrl + X`** 启动。
6. 系统将直接进入免密 root shell，执行：
   ```bash
   # 1. 重新挂载根目录可写
   mount -o remount,rw /

   # 2. 重新设置 root 密码
   passwd root

   # 3. 恢复官方公钥
   mkdir -p /root/.ssh && chmod 700 /root/.ssh
   echo "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDJnFwr928bhbJwlml6dWpb548660kf7GjKg/4mBV/DD4U0rYfa7/Nxdb1n060T8ei+CWPKjS5q67xD0ux2PRy7v3kMnz6RBv8NcnQg+NENGpfWxlyQH3/gu4dMgWfKTgdzU5xQoJxR1GgWfdfFlKe6r3n8K0XA67USevp3gxDu413miiSoLL+L3UL3xo0EHPYeRpH1CNPHQr5nHgk6upEn1fYHkuby7pBhF66TTMS47rj5ClvQJWC1Ighc4Hll34unBbe4y+kZRolAnU6k4FdvhFo6SQSPYfl5dovH8/sYmVGzAario8hktCEvx3M4O0Dguxg8zZuMZbX+/L3Q8kLvr0UDSjpFzqfSVqAcoPFBTI4BfdQzCuDmnQd9c1iXAoYgqX1mTOl1bxwmTL1rfrzftgfVPVoLKAH7ILKhn0zK1Hl2sK8RkMQW0Dvyg9Y09zyMqsEZvFLdgd+cpEcOv4gdbwdTyt92Nb5fyuz6MK/2O01q+pr6q9uokijc5/wizlU= Generated-by-Nova" > /root/.ssh/authorized_keys
   chmod 600 /root/.ssh/authorized_keys

   # 4. 正常启动系统
   exec /sbin/init
   ```

---

## 四、 控制权恢复后的协同处理

一旦 M sir 成功登录目标机器（或通过方案恢复了 SSH 密钥）：
1. **通知我们**：我们将第一时间登录机器排查是否存在恶意提权脚本、木马后门、恶意外连进程。
2. **安全组加固**：请确认华为云安全组中，**节点机（Rpc1、Rpc2、Archive）的 8545 / 8546 端口仅允许 Nginx 内网 IP 访问，绝不暴露给 `0.0.0.0/0`**。
3. **服务重启**：排查完毕后，安全重启 Geth 节点进程，确保链上 RPC 读写服务全部满血恢复。
