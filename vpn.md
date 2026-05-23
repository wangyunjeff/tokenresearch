# VPN Nodes

从用户提供的 Clash 配置中提取的美国、英国、新加坡节点。

## 加入项目的方法

### 原理

- 这个项目的 `proxies` 只支持 `http / https / socks5 / socks5h`。
- 这里的很多节点是 `trojan / ss / vmess`，不能直接写进项目数据库。
- 正确做法是：先用单独的 Clash sidecar 把某个节点转换成本地代理，再把这个本地代理注册进项目。
- 现在已经有现成脚本：`/mnt/data/service_codex2/tools/install_selected_vpn_proxies.py`

### 当前已经接入项目的节点

- `新加坡BGP[M][Trojan][倍率:0.7]` -> `socks5h://127.0.0.1:17901`
- `美国AN[M][Trojan][测试][倍率:0.5]` -> `socks5h://127.0.0.1:17902`
- `上海电信转美国GS5[Trojan][倍率:1]` -> `socks5h://127.0.0.1:17903`
- `英国CV[Trojan][测试][倍率:0.5]` -> `socks5h://127.0.0.1:17904`
- `上海电信转英国CV[Trojan][倍率:1]` -> `socks5h://127.0.0.1:17905`
- `美国BGP3[M][Trojan][倍率:0.6]` -> `socks5h://127.0.0.1:17906`
- `美国BGP[M][Trojan][倍率:0.6]` -> `socks5h://127.0.0.1:17907`
- `美国BGP2[M][Trojan][倍率:0.6]` -> `socks5h://127.0.0.1:17908`
- `上海电信转美国GS6[Trojan][倍率:1]` -> `socks5h://127.0.0.1:17909`
- `上海电信转美国GS7[Trojan][倍率:1]` -> `socks5h://127.0.0.1:17910`
- `深港专线转美国AN[M][Trojan][倍率:2.5]` -> `socks5h://127.0.0.1:17911`
- `上海电信转美国AN[M][Trojan][倍率:1]` -> `socks5h://127.0.0.1:17912`

### 后面换节点怎么做

1. 先把新节点整理到这个 `vpn.md` 里。
2. 打开 `tools/install_selected_vpn_proxies.py`，修改 `SELECTED_SIDECARS`。

示例：

```python
SidecarSpec(
    key="sg_bgp",
    name="新加坡BGP[M][Trojan][倍率:0.7]",
    http_port=17801,
    socks_port=17901,
    controller_port=19091,
),
```

字段说明：

- `key`：sidecar 目录名，对应 `backend/data/proxy_sidecars/<key>`
- `name`：必须和 `vpn.md` 里的节点 `name` 完全一致
- `http_port`：这个 sidecar 的本地 HTTP 代理端口
- `socks_port`：这个 sidecar 的本地 SOCKS5 代理端口，也是项目里真正写入的代理端口
- `controller_port`：这个 sidecar 的 Clash 控制端口

最重要的规则：

- 如果你只是想“替换原来的某个槽位”，就保留原来的 `key` 和端口，只改 `name`
- 这样脚本会重启该 sidecar，并把项目里的那条代理更新到新节点
- 如果你改了 `socks_port`，项目数据库里会新增一条代理，旧的那条还会保留

### 执行命令

```bash
python /mnt/data/service_codex2/tools/install_selected_vpn_proxies.py
```

### 脚本会做什么

- 从 `vpn.md` 里按节点名找到对应配置
- 在 `backend/data/proxy_sidecars/<key>/` 下生成 sidecar 配置
- 启动单独的 Clash 进程
- 把本地 `socks5h://127.0.0.1:<socks_port>` 写入项目数据库
- 如果同一个端口的代理已经存在，就更新；不存在就新建

### 验证方法

看项目数据库里的代理：

```bash
python - <<'PY'
import psycopg
conn=psycopg.connect('host=127.0.0.1 port=5432 user=sub2api password=sub2api dbname=sub2api sslmode=disable')
cur=conn.cursor()
cur.execute("select id,name,protocol,host,port,status from proxies where deleted_at is null order by id")
for row in cur.fetchall():
    print(row)
cur.close()
conn.close()
PY
```

看 sidecar 端口是否监听：

```bash
ss -ltnp | rg '1780[1-9]|1781[0-2]|1790[1-9]|1791[0-2]|1909[1-9]|1910[0-2]'
```

### 相关文件

- 节点清单：`/mnt/data/service_codex2/vpn.md`
- 安装脚本：`/mnt/data/service_codex2/tools/install_selected_vpn_proxies.py`
- 项目数据库配置：`/mnt/data/service_codex2/backend/config.yaml`
- sidecar 目录：`/mnt/data/service_codex2/backend/data/proxy_sidecars/`

### 注意

- 机器重启后，如果这些 sidecar 没有做成 systemd 服务，需要重新运行一次安装脚本
- 如果节点名称在 `vpn.md` 里不存在，脚本会直接报错
- 如果端口被别的程序占用，脚本也会报错，需要换一组未占用端口
- 建议优先复用已有 `key + 端口`，这样不会把数据库里的代理记录越加越多

## 配置片段 1

### 新加坡

```yaml
- {name: 🇸🇬 新加坡Y01 | IEPL | x2, server: a.7.4.k.k.g.r.9.6.sg01-ae5.entry.v51124-3a.qpon, port: 20001, type: ss, cipher: aes-256-gcm, password: 3d5ef9b6-1fd8-3ae8-bc39-2bc2e59b18de, udp: true}
- {name: 🇸🇬 新加坡Y02 | IEPL | x2, server: h.2.a.e.3.7.p.9.6.sg02-ae5.entry.v51124-3a.qpon, port: 20076, type: ss, cipher: aes-256-gcm, password: 3d5ef9b6-1fd8-3ae8-bc39-2bc2e59b18de, udp: true}
- {name: 🇸🇬 新加坡Y03 | IEPL | x2, server: o.7.z.e.c.7.p.9.6.sg03-ae5.entry.v51124-3a.qpon, port: 20081, type: ss, cipher: aes-256-gcm, password: 3d5ef9b6-1fd8-3ae8-bc39-2bc2e59b18de, udp: true}
```

### 美国

```yaml
- {name: 🇺🇲 美国Y01 | IEPL | x1.5, server: h.c.7.m.k.g.p.9.6.us01-ae5.entry.v51124-3a.qpon, port: 20011, type: ss, cipher: aes-256-gcm, password: 3d5ef9b6-1fd8-3ae8-bc39-2bc2e59b18de, udp: true}
- {name: 🇺🇲 美国Y02 | IEPL | x1.5, server: i.2.7.e.3.7.p.9.6.us02-ae5.entry.v51124-3a.qpon, port: 20016, type: ss, cipher: aes-256-gcm, password: 3d5ef9b6-1fd8-3ae8-bc39-2bc2e59b18de, udp: true}
```

### 英国

```yaml
- {name: 🇬🇧 英国Y01, server: o.2.a.e.c.7.o.9.6.gb01-ae5.entry.v51124-3a.qpon, port: 20021, type: ss, cipher: aes-256-gcm, password: 3d5ef9b6-1fd8-3ae8-bc39-2bc2e59b18de, udp: true}
```

## 配置片段 2

来源：`/home/ike/.config/clash/config.yaml`

### 新加坡

```yaml
- name: '新加坡BGP[M][Trojan][倍率:0.7]'
  type: trojan
  server: sg-a1.ccdnadns.cc
  port: 65501
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: cdnsg.douyinvod.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true
```

### 美国

```yaml
- name: '上海电信转美国AN[M][Trojan][倍率:1]'
  type: trojan
  server: jugms1.t4kz6py9.top
  port: 65308
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dl.shcthusan.baidu.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '上海电信转美国GS[Trojan][倍率:1]'
  type: trojan
  server: jugms1.t4kz6py9.top
  port: 65301
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dl.shctusgs.cc.baidu.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '上海电信转美国GS2[Trojan][倍率:1]'
  type: trojan
  server: jugms1.t4kz6py9.top
  port: 65320
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dl.scusgs2.speedtest.cn
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '上海电信转美国GS3[Trojan][倍率:1]'
  type: trojan
  server: jugms1.t4kz6py9.top
  port: 65351
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dl.shctusgs3.cc.baidu.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '上海电信转美国GS4[Trojan][倍率:1]'
  type: trojan
  server: jugms1.t4kz6py9.top
  port: 65324
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dl.scusgs4.baidu.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '上海电信转美国GS5[Trojan][倍率:1]'
  type: trojan
  server: jugms1.t4kz6py9.top
  port: 65501
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dl.shctusgs5.baidu.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '上海电信转美国GS6[Trojan][倍率:1]'
  type: trojan
  server: jugms1.t4kz6py9.top
  port: 65401
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dl.shctusgs6.baidu.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '上海电信转美国GS7[Trojan][倍率:1]'
  type: trojan
  server: jugms1.t4kz6py9.top
  port: 65305
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dl.shctusgs7.baidu.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '深港专线转美国AN[M][Trojan][倍率:2.5]'
  type: trojan
  server: smas-aka5.t4kz6py9.top
  port: 65509
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dl.sgusan.baidu.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '美国AN[M][Trojan][测试][倍率:0.5]'
  type: trojan
  server: us-an1.ccdnadns.cc
  port: 65300
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dl.usan01.cc.tjcct.xyz
  udp: true
  ip-version: ipv6-prefer

- name: '美国BGP[M][Trojan][倍率:0.6]'
  type: trojan
  server: us-bgp1.ccdnadns.cc
  port: 65220
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dlus.douyinvod.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '美国BGP2[M][Trojan][倍率:0.6]'
  type: trojan
  server: us-bgp2.ccdnadns.cc
  port: 65220
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dlus.douyinvod.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '美国BGP3[M][Trojan][倍率:0.6]'
  type: trojan
  server: us-bgp3.ccdnadns.cc
  port: 65221
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dlus.douyinvod.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true
```

### 英国

```yaml
- name: '上海电信转英国CV[Trojan][倍率:1]'
  type: trojan
  server: jugms1.t4kz6py9.top
  port: 65403
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dl.shctukcv.baidu.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true

- name: '英国CV[Trojan][测试][倍率:0.5]'
  type: trojan
  server: uk-cv1.ccdnadns.cc
  port: 65220
  password: 7505735e-f7a5-3317-8884-5e068edb113e
  sni: dluk.douyinvod.com
  udp: true
  ip-version: ipv6-prefer
  skip-cert-verify: true
```
