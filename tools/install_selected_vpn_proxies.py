#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import signal
import socket
import subprocess
import time
import urllib.error
import urllib.request
import argparse
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import psycopg
import yaml


ROOT = Path(__file__).resolve().parents[1]
VPN_MD = ROOT / "vpn.md"
BACKEND_CONFIG = ROOT / "backend" / "config.yaml"
SIDECAR_ROOT = ROOT / "backend" / "data" / "proxy_sidecars"
CLASH_BIN = Path("/home/ike/Downloads/clash/clash-linux-amd64-v1.18.0")


@dataclass(frozen=True)
class SidecarSpec:
    key: str
    name: str
    http_port: int
    socks_port: int
    controller_port: int


SELECTED_SIDECARS = [
    SidecarSpec(
        key="uk_cv_shct",
        name="上海电信转英国CV[Trojan][倍率:1]",
        http_port=17805,
        socks_port=17905,
        controller_port=19095,
    ),
    SidecarSpec(
        key="uk_cv_test",
        name="英国CV[Trojan][测试][倍率:0.5]",
        http_port=17804,
        socks_port=17904,
        controller_port=19094,
    ),
    SidecarSpec(
        key="us_bgp3",
        name="美国BGP3[M][Trojan][倍率:0.6]",
        http_port=17806,
        socks_port=17906,
        controller_port=19096,
    ),
    SidecarSpec(
        key="us_bgp",
        name="美国BGP[M][Trojan][倍率:0.6]",
        http_port=17807,
        socks_port=17907,
        controller_port=19097,
    ),
    SidecarSpec(
        key="us_bgp2",
        name="美国BGP2[M][Trojan][倍率:0.6]",
        http_port=17808,
        socks_port=17908,
        controller_port=19098,
    ),
    SidecarSpec(
        key="us_gs6",
        name="上海电信转美国GS6[Trojan][倍率:1]",
        http_port=17809,
        socks_port=17909,
        controller_port=19099,
    ),
    SidecarSpec(
        key="us_gs7",
        name="上海电信转美国GS7[Trojan][倍率:1]",
        http_port=17810,
        socks_port=17910,
        controller_port=19100,
    ),
    SidecarSpec(
        key="us_an_szhk",
        name="深港专线转美国AN[M][Trojan][倍率:2.5]",
        http_port=17811,
        socks_port=17911,
        controller_port=19101,
    ),
    SidecarSpec(
        key="us_an_shct",
        name="上海电信转美国AN[M][Trojan][倍率:1]",
        http_port=17812,
        socks_port=17912,
        controller_port=19102,
    ),
    SidecarSpec(
        key="us_an_test",
        name="美国AN[M][Trojan][测试][倍率:0.5]",
        http_port=17802,
        socks_port=17902,
        controller_port=19092,
    ),
]


def parse_vpn_markdown(path: Path) -> dict[str, dict[str, Any]]:
    text = path.read_text(encoding="utf-8")
    in_code = False
    code_lang = ""
    block_lines: list[str] = []
    proxies: dict[str, dict[str, Any]] = {}

    for raw_line in text.splitlines():
        line = raw_line.rstrip("\n")
        if line.startswith("```"):
            if not in_code:
                in_code = True
                code_lang = line[3:].strip().split(maxsplit=1)[0].lower()
                block_lines = []
            else:
                block_text = "\n".join(block_lines).strip()
                in_code = False
                lang = code_lang
                code_lang = ""
                if lang and lang not in {"yaml", "yml"}:
                    continue
                if not block_text:
                    continue
                try:
                    loaded = yaml.safe_load(block_text)
                except yaml.YAMLError:
                    continue
                if not isinstance(loaded, list):
                    continue
                for item in loaded:
                    if isinstance(item, dict) and "name" in item:
                        proxies[str(item["name"])] = item
            continue
        if in_code:
            block_lines.append(line)

    return proxies


def load_database_dsn() -> str:
    cfg = yaml.safe_load(BACKEND_CONFIG.read_text(encoding="utf-8"))
    db = cfg["database"]
    return (
        f"host={db['host']} port={db['port']} user={db['user']} "
        f"password={db['password']} dbname={db['dbname']} sslmode={db['sslmode']}"
    )


def is_port_in_use(port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(0.2)
        return sock.connect_ex(("127.0.0.1", port)) == 0


def ensure_port_available(port: int, expected_pid: int | None) -> None:
    if not is_port_in_use(port):
        return
    if expected_pid and pid_is_running(expected_pid):
        return
    raise RuntimeError(f"port {port} is already in use by another process")


def pid_is_running(pid: int) -> bool:
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        return True


def stop_existing_sidecar(pid_file: Path) -> None:
    if not pid_file.exists():
        return
    raw = pid_file.read_text(encoding="utf-8").strip()
    if not raw:
        pid_file.unlink(missing_ok=True)
        return
    pid = int(raw)
    if not pid_is_running(pid):
        pid_file.unlink(missing_ok=True)
        return
    os.kill(pid, signal.SIGTERM)
    deadline = time.time() + 5
    while time.time() < deadline:
        if not pid_is_running(pid):
            pid_file.unlink(missing_ok=True)
            return
        time.sleep(0.2)
    os.kill(pid, signal.SIGKILL)
    deadline = time.time() + 3
    while time.time() < deadline:
        if not pid_is_running(pid):
            pid_file.unlink(missing_ok=True)
            return
        time.sleep(0.1)
    raise RuntimeError(f"failed to stop sidecar pid {pid}")


def build_clash_config(proxy: dict[str, Any], spec: SidecarSpec) -> dict[str, Any]:
    return {
        "port": spec.http_port,
        "socks-port": spec.socks_port,
        "allow-lan": False,
        "mode": "Rule",
        "log-level": "silent",
        "unified-delay": True,
        "external-controller": f"127.0.0.1:{spec.controller_port}",
        "dns": {"enable": False},
        "proxies": [proxy],
        "proxy-groups": [
            {
                "name": "AUTO",
                "type": "select",
                "proxies": [proxy["name"]],
            }
        ],
        "rules": ["MATCH,AUTO"],
    }


def wait_for_controller(controller_port: int, timeout_seconds: int = 15) -> None:
    url = f"http://127.0.0.1:{controller_port}/version"
    deadline = time.time() + timeout_seconds
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=1.5) as response:
                if response.status == 200:
                    return
        except Exception as exc:  # noqa: BLE001
            last_error = exc
            time.sleep(0.3)
    raise RuntimeError(f"controller {controller_port} did not start: {last_error}")


def start_sidecar(proxy: dict[str, Any], spec: SidecarSpec) -> dict[str, Any]:
    sidecar_dir = SIDECAR_ROOT / spec.key
    home_dir = sidecar_dir / "home"
    log_path = sidecar_dir / "clash.log"
    config_path = sidecar_dir / "config.yaml"
    pid_path = sidecar_dir / "clash.pid"
    meta_path = sidecar_dir / "meta.json"

    sidecar_dir.mkdir(parents=True, exist_ok=True)
    home_dir.mkdir(parents=True, exist_ok=True)

    stop_existing_sidecar(pid_path)
    ensure_port_available(spec.http_port, None)
    ensure_port_available(spec.socks_port, None)
    ensure_port_available(spec.controller_port, None)

    config = build_clash_config(proxy, spec)
    config_path.write_text(
        yaml.safe_dump(config, allow_unicode=True, sort_keys=False),
        encoding="utf-8",
    )

    log_handle = log_path.open("w", encoding="utf-8")
    process = subprocess.Popen(
        [str(CLASH_BIN), "-d", str(home_dir), "-f", str(config_path)],
        stdout=log_handle,
        stderr=subprocess.STDOUT,
        start_new_session=True,
        text=True,
    )
    log_handle.close()
    pid_path.write_text(str(process.pid), encoding="utf-8")
    wait_for_controller(spec.controller_port)

    meta = {
        "name": spec.name,
        "http_port": spec.http_port,
        "socks_port": spec.socks_port,
        "controller_port": spec.controller_port,
        "pid": process.pid,
        "config_path": str(config_path),
        "log_path": str(log_path),
    }
    meta_path.write_text(json.dumps(meta, ensure_ascii=False, indent=2), encoding="utf-8")
    return meta


def configure_sidecar(proxy: dict[str, Any], spec: SidecarSpec) -> dict[str, Any]:
    sidecar_dir = SIDECAR_ROOT / spec.key
    home_dir = sidecar_dir / "home"
    log_path = sidecar_dir / "clash.log"
    config_path = sidecar_dir / "config.yaml"
    pid_path = sidecar_dir / "clash.pid"
    meta_path = sidecar_dir / "meta.json"

    sidecar_dir.mkdir(parents=True, exist_ok=True)
    home_dir.mkdir(parents=True, exist_ok=True)

    config = build_clash_config(proxy, spec)
    config_path.write_text(
        yaml.safe_dump(config, allow_unicode=True, sort_keys=False),
        encoding="utf-8",
    )

    pid: int | None = None
    if pid_path.exists():
        raw = pid_path.read_text(encoding="utf-8").strip()
        if raw.isdigit():
            pid = int(raw)

    meta = {
        "name": spec.name,
        "http_port": spec.http_port,
        "socks_port": spec.socks_port,
        "controller_port": spec.controller_port,
        "pid": pid,
        "config_path": str(config_path),
        "log_path": str(log_path),
    }
    meta_path.write_text(json.dumps(meta, ensure_ascii=False, indent=2), encoding="utf-8")
    return meta


def upsert_project_proxy(conn: psycopg.Connection[Any], spec: SidecarSpec) -> dict[str, Any]:
    with conn.cursor() as cur:
        cur.execute(
            """
            select id, name
            from proxies
            where deleted_at is null and host = %s and port = %s
            order by id
            limit 1
            """,
            ("127.0.0.1", spec.socks_port),
        )
        existing = cur.fetchone()
        if existing:
            proxy_id = int(existing[0])
            cur.execute(
                """
                update proxies
                set name = %s,
                    protocol = %s,
                    host = %s,
                    port = %s,
                    username = null,
                    password = null,
                    status = %s,
                    updated_at = now()
                where id = %s
                """,
                (spec.name, "socks5h", "127.0.0.1", spec.socks_port, "active", proxy_id),
            )
            return {"id": proxy_id, "action": "updated"}

        cur.execute(
            """
            insert into proxies (name, protocol, host, port, username, password, status, created_at, updated_at)
            values (%s, %s, %s, %s, null, null, %s, now(), now())
            returning id
            """,
            (spec.name, "socks5h", "127.0.0.1", spec.socks_port, "active"),
        )
        proxy_id = int(cur.fetchone()[0])
        return {"id": proxy_id, "action": "created"}


def verify_local_proxy(spec: SidecarSpec) -> dict[str, Any]:
    command = [
        "curl",
        "--proxy",
        f"socks5h://127.0.0.1:{spec.socks_port}",
        "--connect-timeout",
        "4",
        "--max-time",
        "8",
        "-sS",
        "-o",
        "/dev/null",
        "-w",
        "%{http_code} %{time_total}",
        "https://api.openai.com/v1/models",
    ]
    result = subprocess.run(command, capture_output=True, text=True)
    output = result.stdout.strip()
    parts = output.split()
    return {
        "ok": result.returncode == 0 and len(parts) == 2 and parts[0] != "000",
        "http_code": parts[0] if len(parts) >= 1 else "000",
        "time_total": parts[1] if len(parts) >= 2 else "",
        "stderr": result.stderr.strip(),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--configure-only",
        action="store_true",
        help="write sidecar configs and database rows without starting Clash processes",
    )
    args = parser.parse_args()

    if not CLASH_BIN.exists():
        raise RuntimeError(f"clash binary not found: {CLASH_BIN}")
    SIDECAR_ROOT.mkdir(parents=True, exist_ok=True)

    proxies = parse_vpn_markdown(VPN_MD)
    selected_proxies: list[tuple[SidecarSpec, dict[str, Any]]] = []
    for spec in SELECTED_SIDECARS:
        proxy = proxies.get(spec.name)
        if proxy is None:
            raise RuntimeError(f"proxy not found in vpn.md: {spec.name}")
        selected_proxies.append((spec, proxy))

    started: list[dict[str, Any]] = []
    for spec, proxy in selected_proxies:
        if args.configure_only:
            meta = configure_sidecar(proxy, spec)
            verify = {"skipped": True}
        else:
            meta = start_sidecar(proxy, spec)
            verify = verify_local_proxy(spec)
        started.append({"spec": spec, "meta": meta, "verify": verify})

    dsn = load_database_dsn()
    db_rows: list[dict[str, Any]] = []
    with psycopg.connect(dsn) as conn:
        for item in started:
            result = upsert_project_proxy(conn, item["spec"])
            db_rows.append({"name": item["spec"].name, **result})
        conn.commit()

    verb = "Configured" if args.configure_only else "Installed"
    print(f"{verb} selected VPN proxies:")
    for item in started:
        spec = item["spec"]
        verify = item["verify"]
        print(
            json.dumps(
                {
                    "name": spec.name,
                    "socks5h": f"socks5h://127.0.0.1:{spec.socks_port}",
                    "http": f"http://127.0.0.1:{spec.http_port}",
                    "controller": f"http://127.0.0.1:{spec.controller_port}",
                    "verify": verify,
                },
                ensure_ascii=False,
            )
        )
    print("Database upserts:")
    for row in db_rows:
        print(json.dumps(row, ensure_ascii=False))

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
