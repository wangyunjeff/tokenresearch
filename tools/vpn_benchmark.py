#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import statistics
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from collections import defaultdict
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml


TARGETS = {
    "web": {
        "label": "Normal Web",
        "url": "https://www.wikipedia.org/",
    },
    "openai": {
        "label": "OpenAI Codex",
        "url": "https://api.openai.com/v1/models",
    },
}


def default_clash_bin() -> Path:
    for path in (
        Path("/usr/bin/verge-mihomo"),
        Path("/home/ike/Downloads/clash/clash-linux-amd64-v1.18.0"),
    ):
        if path.exists():
            return path
    return Path("/usr/bin/verge-mihomo")


@dataclass
class ProxyMeta:
    name: str
    fragment: str
    country: str


def parse_vpn_markdown(path: Path) -> tuple[list[dict[str, Any]], dict[str, ProxyMeta]]:
    text = path.read_text(encoding="utf-8")
    proxies: list[dict[str, Any]] = []
    meta: dict[str, ProxyMeta] = {}

    fragment = ""
    country = ""
    in_code = False
    block_lines: list[str] = []

    for raw_line in text.splitlines():
        line = raw_line.rstrip("\n")
        if line.startswith("## "):
            fragment = line[3:].strip()
            continue
        if line.startswith("### "):
            country = line[4:].strip()
            continue
        if line.startswith("```"):
            if not in_code:
                in_code = True
                block_lines = []
            else:
                block_text = "\n".join(block_lines).strip()
                in_code = False
                if not block_text:
                    continue
                loaded = yaml.safe_load(block_text)
                if not isinstance(loaded, list):
                    continue
                for item in loaded:
                    if not isinstance(item, dict) or "name" not in item:
                        continue
                    proxy = dict(item)
                    name = str(proxy["name"])
                    proxies.append(proxy)
                    meta[name] = ProxyMeta(name=name, fragment=fragment, country=country)
            continue
        if in_code:
            block_lines.append(line)

    return proxies, meta


def build_clash_config(proxies: list[dict[str, Any]], path: Path, http_port: int, socks_port: int, controller_port: int) -> None:
    config = {
        "port": http_port,
        "socks-port": socks_port,
        "allow-lan": False,
        "mode": "Rule",
        "log-level": "silent",
        "unified-delay": True,
        "external-controller": f"127.0.0.1:{controller_port}",
        "dns": {"enable": False},
        "proxies": proxies,
        "proxy-groups": [
            {
                "name": "TEST",
                "type": "select",
                "proxies": [proxy["name"] for proxy in proxies],
            }
        ],
        "rules": ["MATCH,TEST"],
    }
    path.write_text(yaml.safe_dump(config, allow_unicode=True, sort_keys=False), encoding="utf-8")


def controller_request(base_url: str, method: str, path: str, payload: dict[str, Any] | None = None) -> Any:
    data = None
    headers = {"Content-Type": "application/json"}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        url=f"{base_url}{path}",
        data=data,
        headers=headers,
        method=method,
    )
    with urllib.request.urlopen(request, timeout=10) as response:
        body = response.read().decode("utf-8")
        return json.loads(body) if body else None


def wait_for_controller(base_url: str, timeout_seconds: int = 20) -> None:
    deadline = time.monotonic() + timeout_seconds
    last_error: Exception | None = None
    while time.monotonic() < deadline:
        try:
            controller_request(base_url, "GET", "/version")
            return
        except Exception as exc:  # noqa: BLE001
            last_error = exc
            time.sleep(0.5)
    raise RuntimeError(f"controller did not start in time: {last_error}")


def select_proxy(base_url: str, proxy_name: str) -> None:
    encoded_group = urllib.parse.quote("TEST", safe="")
    controller_request(base_url, "PUT", f"/proxies/{encoded_group}", {"name": proxy_name})


def run_curl(proxy_url: str, url: str, connect_timeout: int, max_time: int) -> dict[str, Any]:
    template = (
        '{"http_code":"%{http_code}","time_connect":%{time_connect},'
        '"time_starttransfer":%{time_starttransfer},"time_total":%{time_total},'
        '"speed_download":%{speed_download},"size_download":%{size_download},'
        '"remote_ip":"%{remote_ip}","url_effective":"%{url_effective}"}'
    )
    command = [
        "curl",
        "-A",
        "CodexVPNBench/1.0",
        "--proxy",
        proxy_url,
        "--connect-timeout",
        str(connect_timeout),
        "--max-time",
        str(max_time),
        "--compressed",
        "-L",
        "-sS",
        "-o",
        "/dev/null",
        "-w",
        template,
        url,
    ]
    started = time.time()
    result = subprocess.run(command, capture_output=True, text=True)
    finished = time.time()

    metrics: dict[str, Any]
    if result.returncode == 0:
        try:
            metrics = json.loads(result.stdout.strip())
        except json.JSONDecodeError:
            metrics = {"http_code": "000"}
    else:
        metrics = {"http_code": "000"}

    metrics.update(
        {
            "exit_code": result.returncode,
            "stderr": result.stderr.strip(),
            "started_at": started,
            "finished_at": finished,
            "ok": result.returncode == 0 and metrics.get("http_code") not in ("", "000"),
        }
    )
    return metrics


def mean_or_none(values: list[float]) -> float | None:
    return round(statistics.mean(values), 4) if values else None


def normalize_inverse(values: dict[str, float | None]) -> dict[str, float]:
    valid = {key: value for key, value in values.items() if value is not None}
    if not valid:
        return {key: 0.0 for key in values}
    low = min(valid.values())
    high = max(valid.values())
    if abs(high - low) < 1e-9:
        return {key: 1.0 if key in valid else 0.0 for key in values}
    out: dict[str, float] = {}
    for key, value in values.items():
        if value is None:
            out[key] = 0.0
        else:
            out[key] = (high - value) / (high - low)
    return out


def summarize_results(results: list[dict[str, Any]], meta: dict[str, ProxyMeta]) -> dict[str, Any]:
    grouped: dict[str, dict[str, list[dict[str, Any]]]] = defaultdict(lambda: defaultdict(list))
    for row in results:
        grouped[row["node"]][row["target"]].append(row)

    summary: dict[str, Any] = {"nodes": {}}
    web_totals: dict[str, float | None] = {}
    openai_totals: dict[str, float | None] = {}

    for node, by_target in grouped.items():
        node_summary: dict[str, Any] = {
            "country": meta[node].country,
            "fragment": meta[node].fragment,
            "targets": {},
        }
        for target_name in TARGETS:
            rows = by_target.get(target_name, [])
            successes = [row for row in rows if row["ok"]]
            totals = [float(row["time_total"]) for row in successes if row.get("time_total") is not None]
            connects = [float(row["time_connect"]) for row in successes if row.get("time_connect") is not None]
            ttfbs = [float(row["time_starttransfer"]) for row in successes if row.get("time_starttransfer") is not None]
            speeds = [float(row["speed_download"]) for row in successes if row.get("speed_download") is not None]
            success_rate = (len(successes) / len(rows)) if rows else 0.0
            node_summary["targets"][target_name] = {
                "samples": len(rows),
                "successes": len(successes),
                "success_rate": round(success_rate, 4),
                "avg_connect": mean_or_none(connects),
                "avg_ttfb": mean_or_none(ttfbs),
                "avg_total": mean_or_none(totals),
                "avg_speed_download": mean_or_none(speeds),
            }
        summary["nodes"][node] = node_summary
        web_totals[node] = node_summary["targets"]["web"]["avg_total"]
        openai_totals[node] = node_summary["targets"]["openai"]["avg_total"]

    web_latency_score = normalize_inverse(web_totals)
    openai_latency_score = normalize_inverse(openai_totals)

    rankings: list[dict[str, Any]] = []
    for node, node_summary in summary["nodes"].items():
        web_score = 0.6 * node_summary["targets"]["web"]["success_rate"] + 0.4 * web_latency_score[node]
        openai_score = 0.6 * node_summary["targets"]["openai"]["success_rate"] + 0.4 * openai_latency_score[node]
        overall = 0.5 * web_score + 0.5 * openai_score
        entry = {
            "node": node,
            "country": node_summary["country"],
            "fragment": node_summary["fragment"],
            "web_success_rate": node_summary["targets"]["web"]["success_rate"],
            "web_avg_total": node_summary["targets"]["web"]["avg_total"],
            "web_avg_speed_download": node_summary["targets"]["web"]["avg_speed_download"],
            "openai_success_rate": node_summary["targets"]["openai"]["success_rate"],
            "openai_avg_total": node_summary["targets"]["openai"]["avg_total"],
            "web_score": round(web_score, 4),
            "openai_score": round(openai_score, 4),
            "overall_score": round(overall, 4),
            "samples_web": node_summary["targets"]["web"]["samples"],
            "samples_openai": node_summary["targets"]["openai"]["samples"],
        }
        rankings.append(entry)

    summary["rankings"] = {
        "overall": sorted(
            rankings,
            key=lambda item: (-item["overall_score"], -item["openai_success_rate"], item["openai_avg_total"] or 999),
        ),
        "web": sorted(
            rankings,
            key=lambda item: (
                -item["web_success_rate"],
                item["web_avg_total"] if item["web_avg_total"] is not None else 999,
                -(item["web_avg_speed_download"] if item["web_avg_speed_download"] is not None else 0),
            ),
        ),
        "openai": sorted(
            rankings,
            key=lambda item: (-item["openai_success_rate"], item["openai_avg_total"] or 999),
        ),
    }
    return summary


def render_table(headers: list[str], rows: list[list[Any]]) -> str:
    def escape_cell(value: Any) -> str:
        return str(value).replace("|", "\\|").replace("\n", " ")

    out = [
        "| " + " | ".join(escape_cell(header) for header in headers) + " |",
        "| " + " | ".join(["---"] * len(headers)) + " |",
    ]
    for row in rows:
        out.append("| " + " | ".join(escape_cell(cell) for cell in row) + " |")
    return "\n".join(out)


def format_rate(value: float) -> str:
    return f"{value * 100:.1f}%"


def format_seconds(value: float | None) -> str:
    return "-" if value is None else f"{value:.3f}s"


def format_speed(value: float | None) -> str:
    return "-" if value is None else f"{value / 1024:.1f} KB/s"


def write_report(
    report_path: Path,
    raw_path: Path,
    summary: dict[str, Any],
    results: list[dict[str, Any]],
    started_at: float,
    finished_at: float,
    requested_duration: int,
) -> None:
    overall_rows = []
    for idx, item in enumerate(summary["rankings"]["overall"][:10], start=1):
        overall_rows.append(
            [
                idx,
                item["node"],
                item["country"],
                format_rate(item["web_success_rate"]),
                format_seconds(item["web_avg_total"]),
                format_rate(item["openai_success_rate"]),
                format_seconds(item["openai_avg_total"]),
                f"{item['overall_score']:.4f}",
            ]
        )

    web_rows = []
    for idx, item in enumerate(summary["rankings"]["web"][:10], start=1):
        web_rows.append(
            [
                idx,
                item["node"],
                item["country"],
                format_rate(item["web_success_rate"]),
                format_seconds(item["web_avg_total"]),
                format_speed(item["web_avg_speed_download"]),
            ]
        )

    openai_rows = []
    for idx, item in enumerate(summary["rankings"]["openai"][:10], start=1):
        openai_rows.append(
            [
                idx,
                item["node"],
                item["country"],
                format_rate(item["openai_success_rate"]),
                format_seconds(item["openai_avg_total"]),
            ]
        )

    all_rows = []
    for idx, item in enumerate(summary["rankings"]["overall"], start=1):
        all_rows.append(
            [
                idx,
                item["node"],
                item["country"],
                item["fragment"],
                format_rate(item["web_success_rate"]),
                format_seconds(item["web_avg_total"]),
                format_rate(item["openai_success_rate"]),
                format_seconds(item["openai_avg_total"]),
                f"{item['overall_score']:.4f}",
            ]
        )

    report = [
        "# VPN Benchmark Report",
        "",
        f"- Started: {time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(started_at))}",
        f"- Finished: {time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(finished_at))}",
        f"- Requested duration: {requested_duration} seconds",
        f"- Actual duration: {finished_at - started_at:.1f} seconds",
        f"- Nodes sampled: {len(summary['nodes'])}",
        f"- Requests recorded: {len(results)}",
        "- Targets:",
        f"  - Normal Web: {TARGETS['web']['url']}",
        f"  - OpenAI Codex proxy: {TARGETS['openai']['url']}",
        "",
        "## Method",
        "",
        "- Temporary Clash instance on isolated local ports.",
        "- Round-robin over all US/UK/SG nodes extracted from `vpn.md`.",
        "- For each node, one normal-web request and one OpenAI request per round.",
        "- Stability = successful HTTP response ratio (`curl` exit code 0 and HTTP code not `000`).",
        "- Speed = average total response time. For normal web, average download speed is also listed.",
        "- OpenAI endpoint returns a small unauthenticated response, so latency is more meaningful than download throughput.",
        "",
        "## Overall Top 10",
        "",
        render_table(
            ["Rank", "Node", "Country", "Web Stability", "Web Avg", "OpenAI Stability", "OpenAI Avg", "Score"],
            overall_rows,
        ),
        "",
        "## Best For Normal Web",
        "",
        render_table(
            ["Rank", "Node", "Country", "Stability", "Avg Time", "Avg Speed"],
            web_rows,
        ),
        "",
        "## Best For OpenAI Codex",
        "",
        render_table(
            ["Rank", "Node", "Country", "Stability", "Avg Time"],
            openai_rows,
        ),
        "",
        "## Full Results",
        "",
        render_table(
            ["Rank", "Node", "Country", "Source", "Web Stability", "Web Avg", "OpenAI Stability", "OpenAI Avg", "Score"],
            all_rows,
        ),
        "",
        f"Raw JSON: `{raw_path.name}`",
        "",
    ]

    report_path.write_text("\n".join(report), encoding="utf-8")
    raw_path.write_text(
        json.dumps(
            {
                "started_at": started_at,
                "finished_at": finished_at,
                "requested_duration": requested_duration,
                "targets": TARGETS,
                "summary": summary,
                "results": results,
            },
            ensure_ascii=False,
            indent=2,
        ),
        encoding="utf-8",
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="Benchmark vpn.md nodes for web and OpenAI access.")
    parser.add_argument("--vpn-md", type=Path, default=Path("vpn.md"))
    parser.add_argument("--duration", type=int, default=600)
    parser.add_argument("--http-port", type=int, default=17890)
    parser.add_argument("--socks-port", type=int, default=17891)
    parser.add_argument("--controller-port", type=int, default=19090)
    parser.add_argument("--connect-timeout", type=int, default=4)
    parser.add_argument("--max-time", type=int, default=10)
    parser.add_argument("--report", type=Path, default=Path("vpn_benchmark_report.md"))
    parser.add_argument("--raw-json", type=Path, default=Path("vpn_benchmark_raw.json"))
    parser.add_argument("--clash-bin", type=Path, default=default_clash_bin())
    parser.add_argument("--clash-home", type=Path, default=Path("/tmp/codex-clash-bench"))
    args = parser.parse_args()

    proxies, meta = parse_vpn_markdown(args.vpn_md)
    if not proxies:
        raise RuntimeError(f"no proxies parsed from {args.vpn_md}")

    args.clash_home.mkdir(parents=True, exist_ok=True)
    base_url = f"http://127.0.0.1:{args.controller_port}"
    proxy_url = f"http://127.0.0.1:{args.http_port}"
    results: list[dict[str, Any]] = []
    started_at = time.time()

    with tempfile.TemporaryDirectory(prefix="codex-vpn-bench-") as tmp_dir:
        tmp_path = Path(tmp_dir)
        config_path = tmp_path / "config.yaml"
        clash_log_path = tmp_path / "clash.log"
        build_clash_config(proxies, config_path, args.http_port, args.socks_port, args.controller_port)

        clash_log = clash_log_path.open("w", encoding="utf-8")
        process = subprocess.Popen(
            [str(args.clash_bin), "-d", str(args.clash_home), "-f", str(config_path)],
            stdout=clash_log,
            stderr=subprocess.STDOUT,
            text=True,
        )
        try:
            wait_for_controller(base_url)
            deadline = time.monotonic() + args.duration
            round_idx = 0
            print(f"loaded {len(proxies)} proxies; running for {args.duration}s", flush=True)
            while time.monotonic() < deadline:
                round_idx += 1
                for proxy in proxies:
                    if time.monotonic() >= deadline:
                        break
                    node = str(proxy["name"])
                    select_proxy(base_url, node)
                    time.sleep(0.35)
                    samples = []
                    for target_name, target in TARGETS.items():
                        if time.monotonic() >= deadline:
                            break
                        metrics = run_curl(proxy_url, target["url"], args.connect_timeout, args.max_time)
                        row = {
                            "round": round_idx,
                            "node": node,
                            "country": meta[node].country,
                            "fragment": meta[node].fragment,
                            "target": target_name,
                            **metrics,
                        }
                        results.append(row)
                        samples.append(f"{target_name}:{row['http_code']}/{row['time_total']:.3f}s" if row["ok"] else f"{target_name}:FAIL")
                    elapsed = time.time() - started_at
                    print(f"[{elapsed:7.1f}s] round={round_idx:02d} node={node} {' '.join(samples)}", flush=True)
        finally:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)
            clash_log.close()

    finished_at = time.time()
    summary = summarize_results(results, meta)
    write_report(args.report, args.raw_json, summary, results, started_at, finished_at, args.duration)

    print(f"report written to {args.report}", flush=True)
    print(f"raw json written to {args.raw_json}", flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
