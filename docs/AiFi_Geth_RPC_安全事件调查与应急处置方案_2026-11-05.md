# AiFi 链 Geth RPC 安全事件调查与应急处置方案 (2026-11-05)

> **事件级别**：P0 级安全事件  
> **报告时间**：2026-11-05  
> **当前状态**：**已全面处置完成，业务全流程零中断恢复**（双活集群加固完成）  
> **关联复盘**：[`CCC_Geth_RPC_安全事件复盘报告_2026-11-04.md`](file:///Users/abel/Documents/code/chain-tech/AiFi-Chain/aifi_chain/docs/CCC_Geth_RPC_安全事件复盘报告_2026-11-04.md)

---

## 一、 事件核心结论与执行总结

| 阶段 | 任务目标 | 状态 | 实测结果 |
| :--- | :--- | :--- | :--- |
| **阶段 0：即刻止血** | Nginx 全局 Lua 过滤所有 `debug_` 请求 | **已完成** | 成功阻断 `debug_` 任意大小写注入，正常 RPC 返回 200，恶意请求 403 丢弃 |
| **阶段 1：失联机器权限恢复** | 夺回 `Rpc1` 控制权，核实 `Archive-02` | **已完成** | `Rpc1` 在线夺回 root 权限（零重启，持续在线 102 天）；`Archive-02` 确系历史淘汰注销节点 |
| **阶段 2：Geth 模块优雅关闭** | 滚动下线 `Rpc1` & `Rpc2`，剔除 `debug` / `txpool` | **已完成** | 零停机滚动更新完成，Geth 原生剔除危险命名空间，Nginx 双活负载均衡满血恢复 |
| **阶段 3：残留木马与文件清理** | 清理黑客投递的 probe / pprof 垃圾文件 | **已完成** | `/tmp` 及 `/var/tmp` 下所有 probe 缓存已彻底清空，crontab 检查正常 |

---

## 二、 节点状态盘点一览表

| 节点名称 | IP 地址 | 角色 | 处置前状态 | 处置后状态 |
| :--- | :--- | :--- | :--- | :--- |
| **n_AiFi-Nginx** | `166.108.234.200` | 生产入口网关 | 仅拦截 setHead | **部署全局 Lua WAF，阻断全部 `debug_` 方法** |
| **n_AiFi-Rpc1** | `124.243.174.69` | 核心 RPC 生产双活 | SSH 报拒绝，debug/txpool 开启 | **root 控制权夺回；Geth 移除 debug/txpool；双活在线** |
| **n_AiFi-Rpc2** | `188.239.46.173` | 核心 RPC 生产双活 | debug/txpool 开启 | **Geth 移除 debug/txpool；清理残留 pprof；双活在线** |
| **n_AiFi-Archive-01** | `124.243.178.193` | 历史归档节点 | 仅公网 Nginx 暴露 | **本地 Nginx 部署 Lua 阻断；内网专供 Blockscout trace 查询** |
| **n_AiFi-Archive-02** | `190.92.199.31` | 历史节点 | 无法登录 | **已注销释放，确认为淘汰机器，已从资产清单剔除** |
| **n_AiFi-Blockscout** | `166.108.198.55` | 区块链浏览器 | 运行正常 | **正常，走内网 `192.168.0.27:8545` 连接 Archive-01** |

---

## 三、 Geth 模块优雅滚动下线（Rolling Update）全过程纪实

为确保**全网 DApp、交易所、钱包调用业务 100% 零中断**，本次加固采用双活集群轮替滚动方案：

```
[步骤 1: 摘除 Rpc1]
  ├── Nginx upstream 注释 192.168.0.178:8545/8546
  ├── nginx -s reload（全网流量 100% 优雅归入 Rpc2）
  └── 验证 https://rpc.aifiv2.ai 响应正常（高度 23,277,968）

[步骤 2: 改造并重启 Rpc1]
  ├── 备份 /root/.ethereum/config.toml
  ├── 将 HTTPModules 与 WSModules 调整为: ["eth", "net", "web3"]
  ├── systemctl restart aifichain.service
  ├── 本地验证: eth 正常出块；debug_writeMemProfile 与 txpool_status 均报不存在
  └── 清理 /tmp/i20381* 与 /var/tmp/zq9 等攻击残留

[步骤 3: 轮替切流至 Rpc1，摘除 Rpc2]
  ├── Nginx upstream 恢复 192.168.0.178，注释 192.168.0.242
  ├── nginx -s reload（全网流量 100% 优雅归入已加固的 Rpc1）
  └── 验证 https://rpc.aifiv2.ai 响应正常（高度 23,278,176）

[步骤 4: 改造并重启 Rpc2]
  ├── 备份 /root/.ethereum/config.toml
  ├── 将 HTTPModules 与 WSModules 调整为: ["eth", "net", "web3"]
  ├── systemctl restart aifichain.service
  ├── 本地验证: eth 正常出块；debug 与 txpool 彻底注销
  └── 清理 /tmp 目录下所有探测文件

[步骤 5: 全量双活恢复]
  ├── Nginx upstream 恢复 192.168.0.242
  └── nginx -s reload（Rpc1 + Rpc2 双活负载均衡满血恢复）
```

---

## 四、 最终端到端验证结果

在公网直接对生产入口 `https://rpc.aifiv2.ai` 发起验收探测：

1. **核心读取（`eth_blockNumber`）**：
   ```json
   {"jsonrpc":"2.0","id":1,"result":"0x1633328"}
   ```
   👉 **状态**：返回 HTTP 200，最新区块高速同步，业务无损。

2. **高危注入（`debug_writeMemProfile`）**：
   ```json
   {"jsonrpc":"2.0","error":{"code":-32601,"message":"rpc method blocked."},"id":null}
   ```
   👉 **状态**：返回 HTTP 403 Forbidden（Nginx WAF 首道防线秒级拦截）。若绕过 Nginx 直接打到 Geth，Geth 同样报 `the method debug_writeMemProfile does not exist`（第二道防线彻底免疫）。

3. **内存排查与状态泄漏（`txpool_status`）**：
   ```json
   {"jsonrpc":"2.0","id":3,"error":{"code":-32601,"message":"the method txpool_status does not exist/is not available"}}
   ```
   👉 **状态**：Geth 层面彻底关闭，拒绝任何未授权的交易池监控与扫池。
