<!--
文档名称：Geth Debug RPC 被滥用导致服务器关键文件覆写：安全事件复盘报告
版本：v1.0
日期：2026-11-04
本 Markdown 根据同名 Word 正式报告转换，保留正文、表格、章节和证据编号。
-->

**事 件 复 盘  /  INCIDENT POSTMORTEM**

# Geth Debug RPC 被滥用 导致服务器关键文件覆写

**CCC / ConsciousnessChain 节点 · 技术安全调查报告**

2026 年 10 月 25 日异常事件  |  截至 2026 年 11 月 4 日的证据复盘

| 字段 | 内容 |
| --- | --- |
| 事件主机 | conscious-achive-node（华为云 ECS） |
| 主要事件日期 | 2026-10-25 19:36—21:08（UTC+08:00，服务器记录） |
| 事件类型 | 受信任 RPC 调试能力被滥用；高权限文件路径覆写 |
| 建议风险级别 | 高风险（关键认证与系统配置被破坏） |
| 调查状态 | 写入机制高置信；入口与人员/IP 归因仍待验证 |
| 文档版本 | v1.0 · 技术复盘稿 · 内部使用 |

**核心判断**

本次异常文件并非被植入的“gzip 恶意程序”。多份文件是有效的 Go pprof 堆内存分析数据，并包含 Geth 的 WriteMemProfile 与 RPC 回调调用链。最符合证据的解释是：某主体调用 Geth 调试 RPC，把输出路径指定到系统敏感文件，导致其原有内容被覆盖。

文档刻意区分：① 10 月 25 日原主机的实际文件覆写；② 11 月 3 日为恢复而分离数据盘后导致的服务启动失败；③ 11 月 3 日其他 RPC 请求与探测活动。三者不能直接画等号。

> 资料基础：ir-check-latest.txt、ir-rpc-profile.txt、ir-next-leads.txt、ir-rpc-config.txt、ir-followup-20261104-001958.txt、log.zip、IP 排行统计及既有对话运维记录。文内使用 [E#] 标记证据出处。

## 01  执行摘要

**结论：**这是一宗可导致高权限文件覆写的 RPC 调试接口滥用事件；不是已被证明的 SSH 入侵或传统勒索加密事件。

- 已证实：/etc/crontab 与多处 /etc、/root 路径在 10 月 25 日被写成有效 GZIP 数据；Cron 在 19:37:01 报语法错误，证明系统配置实际失效。[E1]

- 高置信机制：GZIP 解压数据及字符串中出现 runtime/pprof.writeHeap → Geth internal/debug.WriteMemProfile → RPC handler/callback 路径，符合 debug_writeMemProfile 生成堆快照的行为。[E2][O1]

- 关键放大因素：原 hardwood.service 以 User=root 运行 /usr/local/bin/geth。若调试 RPC 可达，则文件写入继承 root 的文件系统权限。[E3]

- 未证实：10 月 25 日请求来自哪个 IP、经 HTTP/WS/IPC 哪种链路、是否存在完整 root Shell 控制、是否有敏感密钥读取或链上资产损失。

- 恢复阶段：/data 数据盘是应急处置中主动分离的；随后 Geth 与 Nginx 因找不到 /data 下的配置或日志目录而启动失败，不能当成入侵者修改服务的直接证据。[E3][E1]

### 证据可信度矩阵

| 判断事项 | 证据等级 | 依据及边界 |
| --- | --- | --- |
| 关键路径被写为 GZIP | 确认 | 文件类型、字节数、mtime、SHA256 与 Cron 报错均留有记录 |
| Geth 内存分析 RPC 生成 | 高置信 | 堆样本含 WriteMemProfile + RPC 处理函数链；无当日逐请求日志 |
| 调试 API 曾被调用（6/28） | 高置信 | pentest_probe.prof 出现同类函数链；操作者未确认 |
| Geth 以 root 运行 | 确认（配置） | hardwood.service 显式 User=root；10/25 是否同配置需进一步审计 |
| 公网入口造成 10/25 覆写 | 待证实 | 有 Nginx 反代配置，缺 10/25 access 及当日 API 配置 |
| 未知主体已获得 root Shell | 无充分证据 | 文件写入能力与系统交互式控制不是同一概念 |

## 02  事件对象、系统背景与边界

本报告主线是 conscious-achive-node。该服务器部署了基于 Geth 的链节点，systemd 服务 hardwood.service 调用 /usr/local/bin/geth，并使用 /data/.ethereum 作为数据目录。Nginx 配置存在 443/TLS 到本机 8545（HTTP）与 8546（WebSocket）的反向代理。[E3][E4]

配置中可见 archive.consciousnesschain.com → geth_rpc → 127.0.0.1:8545；wss.consciousnesschain.com → 127.0.0.1:8546；rpc.consciousnesschain.com 的配置文件带 .bak 后缀，不能据此认为它当时处于启用状态。当前截取的配置也不能证明 10 月 25 日的实际公开 API 白名单。[E4]

11 月 3 日另一份 CCC RPC 节点事件材料提到不同主机的调试 API 探测与卡块现象。本报告仅在末尾列为关联线索，未经跨主机原始日志核验，不把它与 10 月 25 日主机覆写合并为同一事件。[E7]

## 03  证据时间线（均为 UTC+08:00）

| 时间 | 观察到的证据 | 研判 |
| --- | --- | --- |
| 2026-06-28 09:25 | /root、/root/.ethereum、/home 存在 pentest_probe.prof 性能分析文件 | 同类调试功能曾较早被调用；归属未定 |
| 10-25 19:36:38—19:36:44 | passwd.save、shadow.save、/etc/crontab、SSH 密钥路径被写为 GZIP | 首批针对身份/配置路径的文件写入 |
| 10-25 19:36:54 | /home/consensus 形成同类 GZIP 文件 | 扩展至业务/用户路径 |
| 10-25 19:37:01 | cron 重新读取 /etc/crontab，报告 bad minute 并忽略该文件 | 配置损坏造成即时服务功能异常 |
| 10-25 19:48:31—19:48:42 | gaa18848_rootproof、.gaa18848x 在 /etc、/root、SSH、Ethereum 路径出现 | 疑似写入能力/权限测试；名称不等于归因 |
| 10-25 20:43:08 | fwupd-refresh 失败，status=217/USER | 与账号数据库异常相容，不能单独证明原因 |
| 10-25 20:57—21:08 | /root/ailiquid、/root/geth、/root/node、.ethereum/geth 等同类数据出现 | 持续针对不同目标路径写入 |
| 11-03 19:02—19:08 | /home/geth、/root/i22671_probe.out 形成 GZIP 文件 | 11 月 3 日另有写入活动；需区分自测与未知调用 |
| 11-03 晚间 | 通过 VNC/救援过程恢复登录，并主动分离 /data | 人为应急处置，不属于原始攻击事件 |
| 11-04 00:09—00:19 | 报告显示仅 40GB 系统盘；hardwood 不断重启，缺少 config.toml | 数据盘分离引发的正常依赖缺失；服务本身未证实恶意 |

> 时间线依据：[E1][E3]；其中部分文件的 ctime 在 11 月 3 日救援操作中变化，历史判读使用记录中的 mtime，且 mtime 并非绝对不可伪造。报告采集时 timedatectl 显示系统时钟未同步，跨系统精确关联需校时。[E1]

## 04  GZIP 数据的真正含义：不是恶意软件本体，而是 pprof 输出

早期看到 /etc/crontab、/etc/passwd.save、/root/.ssh/id_rsa 等位置出现大小约 996 KB—1.01 MB 的 gzip 文件，容易把它们误判为“被投放的恶意压缩包”。但这不是一个普通归档文件的命名或正常系统行为。其压缩内容具有 protobuf/pprof 结构特征，且多次包含 Geth 内存分析函数、RPC 框架调用栈。[E1][E2]

### 4.1  最关键的调用链

```text
runtime/pprof.writeHeap
runtime/pprof.(*Profile).WriteTo
github.com/ethereum/go-ethereum/internal/debug.writeProfile
github.com/ethereum/go-ethereum/internal/debug.(*HandlerT).WriteMemProfile
reflect.Value.Call
github.com/ethereum/go-ethereum/rpc.(*callback).call
github.com/ethereum/go-ethereum/rpc.(*handler).runMethod
```

上述字符串实际出现在 /etc/crontab、/root/geth、/root/pentest_probe.prof 等样本中。[E2] 官方 debug 命名空间文档明确将 debug_writeMemProfile 描述为“将内存分配分析写入指定文件”，参数是文件路径。[O1] 因而最符合证据的机制是：调用方提供一个系统文件路径，Geth 在其进程权限下创建或截断目标，再写入堆分析数据。

重要边界：pprof 样本内含 RPC 框架调用函数，强烈指向 RPC 途径；但仅凭该文件不能区分 HTTP、WebSocket 或本地 IPC 的请求来源，也无法直接还原发送者 IP 或完整 JSON 请求体。

### 4.2  推定的攻击路径与实际结果

| 阶段 | 行为与依赖 | 证据状态 |
| --- | --- | --- |
| 入口 | 某客户端可访问暴露了 debug 的 Geth RPC（公网反代 / 内网 / 本地均待验证） | 推定 |
| 调用 | 发起 debug_writeMemProfile，并传入目标文件路径 | 高置信 |
| 权限 | Geth 原服务以 root 运行，文件写入权限范围过大 | 配置确认 |
| 效果 | 文件被堆快照内容覆盖；/etc/crontab 无法解析 | 确认 |
| 后果 | 定时任务配置失效、认证/SSH 文件可能受损，恢复成本高 | 部分确认 |

## 05  系统文件、命令与服务的影响范围

| 对象/位置 | 确认的情况 | 影响与定性 |
| --- | --- | --- |
| /etc/crontab | 2026-10-25 19:36:41 变成 GZIP；19:37 Cron 报错 | 系统级 Cron 文件被破坏，确定性影响 |
| /etc/passwd.save、/etc/shadow.save | 19:36:38、19:36:39 同类 GZIP；活动账户文件已在救援中恢复 | 身份数据库相关路径受到攻击；原始认证状态需历史对比 |
| /root/.ssh/id_rsa | 19:36:44 同类 GZIP | root 私钥路径的原内容可能丢失；不能推断已被读取 |
| /root/ssh-evidence/.ssh/authorized_keys | 19:36:43 的历史样本是 GZIP | 旧 SSH 授权键的历史证据；现用键为后续重新配置 |
| /root/geth、/root/node | 20:57—21:08 同类 GZIP | 被命名为 geth/node 的路径遭写入；不等于 /usr/local/bin/geth、/usr/bin/node 已被替换 |
| /root/.ethereum/keystore、/home/ubuntu | 同类文件写入 | 文件名具有敏感含义，不等于已确认有效钱包/用户目录被覆盖 |
| hardwood.service | 原 Geth systemd 服务；服务历史早于事件 | 无证据证明服务单元被攻击者新建或改写 |
| Nginx 与 Geth 11/3 重启失败 | 缺失 /data/wwwlogs、/data/.ethereum/config.toml | 原数据盘应急分离后的依赖缺失，不能归为攻击行为 |

“系统命令被恶意修改”应拆开表述：已确认存在多个位于系统目录或以命令名命名的路径被覆写；尚未取得 /usr/bin、/bin 或 /usr/local/bin 下关键可执行程序被篡改的直接校验结果。取证报告中的 /usr/local/bin/geth 仍是原服务启动目标。[E1][E3]

## 06  入口分析、可归因范围与日志缺口

### 6.1  现有反向代理拓扑

Nginx 配置显示 443 HTTPS 入口经 upstream 转发至本地 8545，另有 WebSocket 入口转发至 8546。配置摘录没有显示上述 location 内直接配置身份认证或 allow/deny，但仍需检查完整 nginx -T、负载均衡/CDN/WAF、云安全组，以及 Geth 当时的 HTTPModules/WSModules。[E4]

Geth 官方文档说明，HTTP 默认开放 eth、net、web3，debug 等附加命名空间需要主动启用；因此不能仅因出现 8545/8546 反代就认定 debug 已对公网开放。[O2] 但当日已有的文件写入效果意味着至少存在一个可执行该能力的 RPC 调用面。

### 6.2  为什么目前没有找到 10 月 25 日的攻击 IP

已有 log.zip 的 access 文件覆盖 2026-10-29 至 2026-11-03（11 月 3 日截至 19:45:19）；总计 55,755 条、858 个来源 IP，不含 10 月 25 日主案的 HTTP 访问记录。10 月 25 日的 error.log.2.gz 仅有 19:07:18 /50x.html 访问失败一条，无法用于定位 19:36 之后的 RPC 调用。[E5]

| 已统计来源 | 请求量 | 适用范围与警示 |
| --- | --- | --- |
| 187.124.137.225 | 38,448 | 集中于 11/03，存在大量自动化 RPC 调用与探测；不是 10/25 的归因证据 |
| 18.141.119.157 | 8,385 | Uptime-Kuma 探活；用户此前已确认是自有来源 |
| 14.187.151.8 | 2,145 | 普通请求量排序信息；没有直接关联 10/25 文件写入 |

普通 Nginx access log 通常记录请求路径、时间、状态码、User-Agent，不保存 JSON-RPC POST body；因此即使找回 10 月 25 日的 access 日志，也不一定能直接看到 debug_writeMemProfile 方法名。必须结合 Geth 自身请求日志、WAF/网关审计、代理时间戳以及可能的追踪日志进行归因。

### 6.3  已排查的替代假设

- SSH 登录爆破：现有 10/25 19:30—21:15 auth/journal 片段未显示对应的远程 SSH 登录链路；缺乏这类证据不代表完全排除。用户已确认历史记录中若干 IP 是本人控制。

- Cron 自动改写：19:35 执行的是正常 sysstat 任务；19:37 报的是文件已损坏后的读取错误，并非写入者身份。

- 华为云 Agent：Uniagent / Telescope 日志多为心跳、采集与配置查询；未见直接覆写证明。19:36:39 配置拉取与文件修改时间相近，但相近不能构成因果。

- hardwood.service：其 ExecStart 指向 /usr/local/bin/geth 且服务文件存在较长历史。11/03 缺失 /data 所致反复失败应从攻击证据中剔除。

## 07  调查与应急处置复盘

### 7.1  当时为什么会出现“无法 SSH / 无法正常启动”

多处认证相关文件被覆写后，服务器的系统身份信息和 SSH 认证路径出现异常。应急期间又有意将独立数据盘从旧 ECS 分离：原 /etc/fstab 中依赖该盘的挂载项一度阻碍启动，后经救援修改得以继续。随后通过 VNC / init=/bin/bash 救援、使用原有的账户文件备份与重新配置 SSH 公钥恢复了管理访问。

这两个因素必须分开：原始文件覆写属于 10/25 主案；分离数据盘后出现的 /data 挂载缺失、Nginx 日志路径不存在、Geth 配置无法打开，是 11/03 恢复过程引入的可预期现象。

### 7.2  已做的正确处置

- 把原始 /data 数据盘从旧系统隔离，避免疑似受损系统继续触碰业务数据。

- 对系统盘进行只读采样、文件元信息采集、SHA256 与日志导出；未将样本文件直接解压覆盖原路径。

- 通过复核实际二进制内容，把最初“未知 gzip / 恶意二进制”假设修正为 Geth 堆内存分析文件写入。

- 按实际业务上下文识别 hardwood.service，避免将原有 Geth 服务误报为恶意持久化。

### 7.3  当前操作上的注意点

旧 ECS 应视为不可信取证环境，不建议直接恢复业务或接回原始数据盘。任何用于调试的私钥、RPC 管理令牌、SSH 密钥和运维凭据均应在干净环境下评估轮换；但“存在文件覆写”不是“已确认密钥外泄”，影响通知应按证据分级。

## 08  根因树：技术缺陷与管理缺口

| 层次 | 原因/条件 | 对应修复方向 |
| --- | --- | --- |
| 直接机制 | Geth debug_writeMemProfile 接收可控文件路径并写入堆快照 | 拒绝对外暴露可写文件的 debug 方法 |
| 权限放大 | hardwood.service 配置 User=root；调试写入继承高权限 | Geth 进程改为独立非特权账户 |
| 暴露面 | 443 → 8545/8546 存在代理链路；当日完整 API 授权与网关策略未知 | 公共 RPC 与管理 RPC 物理/逻辑分离 |
| 检测盲区 | 无 auditd；缺 10/25 access 请求正文、Geth RPC 明细 | 集中日志、RPC 方法审计、进程/文件监控 |
| 业务恢复依赖 | 配置、日志与链数据在 /data；节点开机自动重启 | 完善依赖检测、只读恢复和故障切换 |

本案应归类为“高权限节点上的敏感调试接口被滥用导致文件系统完整性破坏”。现阶段证据不能断言是否另有操作系统漏洞、SSH 入侵、root Shell、横向移动或链上盗币。

## 09  修复优先级与落地行动清单

### P0  立即控制风险（所有对外 RPC 节点）

- 清点每台 Geth 的启动命令、config.toml、HTTPModules、WSModules、IPC 及外部 Nginx / LB；特别检查 debug、admin、personal、miner、txpool 等管理或敏感能力。

- 公共 HTTP/WS 命名空间按业务需求最小化；调试/写文件类 API 不经公网暴露。仅靠云安全组关闭 8545/8546 不够，因为 443 反代仍可能转发请求。

- 对 RPC 公网入口添加身份/速率/并发控制；使用能解析 JSON-RPC 单请求和 batch 的网关策略，WebSocket 需逐消息控制。不要把 Nginx 对 $request_body 的简单正则当作唯一屏障。

- 对疑似受到调试写文件影响的机器先下线或隔离，保存系统盘和数据盘的快照及云端操作审计；停止可持续写入的高风险暴露面。

### P1  安全恢复与影响核实

- 在华为云上保存旧系统盘与原数据盘快照；最好由数据盘快照创建克隆盘，连接同可用区的全新干净 ECS，优先只读挂载并复制所需日志/数据。

- 分别核对 /data/.ethereum/config.toml、Geth 自身历史日志、/data/wwwlogs/archive/、/data/wwwlogs/wss/、其他代理/CDN/WAF 日志，重点覆盖 10/25 19:30—21:15。

- 将公钥、密钥、服务配置、二进制、cron、systemd 单元与可信部署版本逐项对比；区分“探测创建文件”与“覆盖了原有有效文件”。

- 恢复生产使用新镜像、新独立运行账号、经验证的链数据/备份和最小化配置；不得把旧机的整套系统目录原封不动复制到新机。

### P2  长期加固与检测

- 将 Geth 改为独立非 root 用户，限制可写目录，并通过 systemd 的 ProtectSystem、ReadWritePaths、NoNewPrivileges 等沙箱机制按兼容性逐项验证。

- 集中保留 Nginx/Geth 访问审计和云平台管理事件，保留期覆盖排查需求；对关键路径的异常创建/改写设置告警与审计。

- 为 RPC 建立资源配额、超时、请求体限制与细粒度命名空间/方法白名单；进行安全回归测试。

- 演练“无数据盘启动”“Geth 配置丢失”“关键认证文件损坏”的恢复场景，记录可复现的 RTO/RPO。

## 10  尚待解决的问题与下一份证据

| 未解决问题 | 最有价值的证据 | 可得结论 |
| --- | --- | --- |
| 10/25 来源 IP 和访问入口？ | 当日 Nginx/access、CDN/WAF/LB 日志、Geth RPC 请求日志 | HTTP/WS 请求来源和时间关联 |
| 10/25 debug API 如何开放？ | 当日配置快照、启动参数、历史部署记录、云安全组变更 | 区分公网反代、内网和 IPC |
| 6/28 pentest_probe 是否己方测试？ | 部署流水线、运维工单、当日控制台日志 | 判断是较早探测、测试还是未知行为 |
| 凭据/Keystore 是否被读取？ | Geth RPC 读文件错误日志、网络流量、钱包操作证据 | 不能只凭被覆写路径推断外泄 |
| 11/3 187.124.137.225 是否与 10/25 同一主体？ | 两个时段可对照的代理与 Geth 日志、云平台审计 | 不能用后一天的 IP 反推更早主案 |
| 关键系统服务是否被改写？ | systemd 文件哈希、包校验、可信部署版本和系统盘快照 | 将正常服务故障与恶意改写区分 |

建议下一轮先不对旧机运行新的高负载排查脚本，而是优先找回 10/25 的网络与 Geth 业务日志，以及原始系统盘、数据盘的只读镜像。当前证据已能解释“怎么写坏的”，缺的主要是“从哪里调用、是谁调用”。

## 附录 A  关键原始证据摘录

### A1  文件类型与哈希

/etc/crontab: gzip compressed data，1005826 bytes；mtime=2026-10-25 19:36:41.073+08:00；SHA256=7ccddba231e97db086c416e6f69dbfae3371f826857ff87e8b40c07913d6effb。[E1]

/root/.ssh/id_rsa: gzip compressed data，1005991 bytes；mtime=2026-10-25 19:36:44.342+08:00；SHA256=3757ba2eecafd23885560367df138aa375fc22f8ca77bdf32835212e1124bbf2。[E1]

GZIP 解压后 /etc/crontab 的前几个字节：5a 04 08 01 10 02 60 80 80 20…；部分函数名可直接检出 WriteMemProfile / runtime/pprof.writeHeap。[E1][E2]

### A2  系统日志事件

```text
2026-10-25T19:37:01+08:00 cron: RELOAD (/etc/crontab)
2026-10-25T19:37:01+08:00 cron: Error: bad minute; while reading /etc/crontab
2026-10-25T19:37:01+08:00 cron: Syntax error, this crontab file will be ignored
2026-11-04 00:19 geth: Fatal: open /data/.ethereum/config.toml: no such file or directory
```

### A3  样本调用链（语义缩写）

```text
runtime/pprof.writeHeap
→ go-ethereum/internal/debug.writeProfile
→ (*HandlerT).WriteMemProfile
→ rpc.(*callback).call → rpc.(*handler).runMethod
```

## 附录 B  来源、复核路径与参考文献

| 编号 | 证据/文档 | 内容与限制 |
| --- | --- | --- |
| E1 | ir-check-latest.txt（2026-11-04 00:09） | 文件元数据/哈希、Cron/journal、账户、审计能力与存储状态；调查期间已经进行了救援操作 |
| E2 | ir-rpc-profile.txt | 样本解压字符串与 Geth WriteMemProfile/RPC 调用链；未提供原始完整 .prof 二进制 |
| E3 | ir-followup-20261104-001958.txt | hardwood.service 配置、Geth 依赖 /data 与反复启动失败 |
| E4 | ir-next-leads.txt；ir-rpc-config.txt | Nginx HTTP/WS 反向代理、配置文件名与日志目录；并非当日完整 nginx -T |
| E5 | log.zip；nginx_ip_requests_ranking_2026-10-29_to_2026-11-03.txt | 10/29—11/03 access 日志；缺少 10/25 主案日志；统计已复核 |
| E7 | incident_report_20261103_ccc_rpc.md | 另一节点 11/03 事件的既有总结，属于二手报告，不能直接并案 |
| O1 | geth.ethereum.org/docs/interacting-with-geth/rpc/ns-debug | Geth 官方 debug_writeMemProfile 方法说明 |
| O2 | geth.ethereum.org/docs/interacting-with-geth/rpc | Geth 官方 RPC transport / HTTP 默认命名空间与 API 白名单 |
| O3 | geth.ethereum.org/docs/fundamentals/security | Geth 官方节点 RPC 与网络安全指导 |

> 在线参考页面访问日期：2026-11-04。因官方站点文档可持续更新，归档时建议同时保存官方页面快照。

## 附录 C  与 11 月 3 日其他 RPC 探测的关系（不并案）

既有 CCC RPC 节点总结称，11/03 另有来源 187.124.137.225 发起 debug_writeMemProfile、debug_traceBlockFromFile 等探测；其中部分路径访问失败、/proc/self/mem 写入返回 I/O 错误。该材料还将另外的 RPC 节点卡块与这些调用相关联。[E7]

但 [E7] 的主机身份和时间与 10/25 conscious-achive-node 主案不同，且目前只取得总结、未取得当日对应 Geth 原始日志。可以将其作为“可能存在持续的 RPC 调试接口暴露/探测”的预警，不能据此认定 10/25 攻击源，也不能将另一节点的因果解释直接移植到本案。

## 最终结论

本案已经从“异常 gzip 文件导致系统损坏”的表象，推进到“Geth 调试内存分析接口具有指定路径写文件能力、且高权限进程使其能触及关键系统文件”的技术机制。10 月 25 日的关键路径覆写及 Cron 故障有充分证据，Geth WriteMemProfile/RPC 机制具有高度解释力。

尚未完成的是外部入口验证与请求归因。现阶段最佳后续动作是隔离与保全旧环境、对所有公网 RPC 去除高危调试能力、以非 root 重建节点，再从 10 月 25 日的网络、Geth 与云平台日志定位调用方。报告应随新增原始日志持续修订，而不提前归责于特定 IP 或认定密钥已泄露。
