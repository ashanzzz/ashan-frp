#!/usr/bin/env python3
"""
1Panel CLI Management Tool for Ashan FRP Project
Provides direct terminal commands to list sites, create custom-port proxies,
apply Let's Encrypt certificates, and inspect 1Panel MCP status.
"""

import os
import sys
import json
import time
import hashlib
import argparse
import urllib.request
import urllib.parse
import urllib.error
from typing import Dict, Any, Optional

def load_env(path: str = "/opt/data/.env") -> Dict[str, str]:
    env = {}
    if not os.path.exists(path):
        return env
    with open(path) as f:
        for line in f:
            line = line.rstrip("\n")
            if line.startswith("#") or "=" not in line:
                continue
            k, v = line.split("=", 1)
            env[k.strip()] = v.strip().strip('"').strip("'")
    return env

def request_1panel(method: str, path: str, body: Optional[Dict[str, Any]] = None, query: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
    env = load_env()
    base_url = env.get("ONEPANEL_BASE_URL", "http://192.168.8.11:10086/").rstrip("/")
    api_key = env.get("ONEPANEL_API_KEY", "")
    if not api_key:
        raise ValueError("Missing ONEPANEL_API_KEY in .env")

    ts = str(int(time.time()))
    sig_raw = f"1panel{api_key}{ts}"
    token = hashlib.md5(sig_raw.encode("utf-8")).hexdigest()

    normalized_path = path if path.startswith("/") else f"/{path}"
    url = f"{base_url}{normalized_path}"

    if query:
        q_params = [f"{k}={urllib.parse.quote(str(v))}" for k, v in query.items() if v is not None and v != ""]
        if q_params:
            url += "?" + "&".join(q_params)

    headers = {
        "Content-Type": "application/json",
        "Accept": "application/json, text/plain, */*",
        "1Panel-Token": token,
        "1Panel-Timestamp": ts,
    }

    data_bytes = None
    if body is not None:
        data_bytes = json.dumps(body).encode("utf-8")
        headers["Content-Length"] = str(len(data_bytes))

    req = urllib.request.Request(url, data=data_bytes, headers=headers, method=method.upper())

    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            raw = resp.read().decode("utf-8", errors="ignore")
            try:
                return json.loads(raw)
            except Exception:
                return {"code": resp.status, "raw": raw}
    except urllib.error.HTTPError as e:
        err_body = e.read().decode("utf-8", errors="ignore")
        return {"code": e.code, "error": str(e), "body": err_body}
    except Exception as e:
        return {"code": 500, "error": str(e)}

def cmd_list_sites(args):
    res = request_1panel("POST", "/api/v2/websites/search", body={
        "page": 1,
        "pageSize": 50,
        "name": args.name or "",
        "orderBy": "created_at",
        "order": "descending"
    })
    items = res.get("data", {}).get("items", [])
    print(f"Total websites: {len(items)}")
    for item in items:
        ssl_info = f"SSL: {item.get('sslStatus')} (exp: {item.get('sslExpireDate', '')[:10]})" if item.get('protocol') == 'HTTPS' else "No SSL"
        print(f"[{item.get('id')}] {item.get('alias')} ({item.get('primaryDomain')}) | Port/Remark: {item.get('remark')} | {item.get('status')} | {ssl_info}")

def cmd_create_proxy(args):
    print(f"Creating 1Panel proxy website for '{args.alias}'...")
    print(f" - Local Listening Port: {args.port} (on 192.168.8.11)")
    print(f" - Backend Proxy Target: {args.proxy_target}")
    print(f" - SSL Enabled: {not args.no_ssl} (SSL ID: {args.ssl_id})")

    payload = {
        "primaryDomain": args.primary_domain or args.alias,
        "alias": args.alias,
        "type": "proxy",
        "appType": "installed",
        "webSiteGroupID": 1,
        "proxy": args.proxy_target,
        "proxyType": "",
        "domains": [
            {
                "domain": "192.168.8.11",
                "port": args.port
            }
        ],
        "enableSSL": not args.no_ssl,
        "websiteSSLID": args.ssl_id,
        "remark": f"{args.port} {args.alias}"
    }
    res = request_1panel("POST", "/api/v2/websites", body=payload)
    print("Result:", json.dumps(res, ensure_ascii=False, indent=2))

def cmd_apply_ssl(args):
    print(f"Applying Let's Encrypt certificate for '{args.domain}' via Cloudflare DNS challenge...")
    payload = {
        "primaryDomain": args.domain,
        "provider": "dnsAccount",
        "acmeAccountId": args.acme_id,
        "dnsAccountId": args.dns_id,
        "autoRenew": True,
        "keyType": "EC256",
        "apply": True
    }
    res = request_1panel("POST", "/api/v2/websites/ssl", body=payload)
    print("Result:", json.dumps(res, ensure_ascii=False, indent=2))

def cmd_list_certs(args):
    res = request_1panel("POST", "/api/v2/websites/ssl/search", body={"page": 1, "pageSize": 50})
    items = res.get("data", {}).get("items", [])
    print(f"Total certificates: {len(items)}")
    for item in items:
        print(f"[{item.get('id')}] {item.get('primaryDomain')} | Type: {item.get('keyType')} | Status: {item.get('status')} | Expire: {item.get('expireDate', '')[:10]}")

def main():
    parser = argparse.ArgumentParser(description="1Panel CLI Operations Tool for Ashan FRP")
    subparsers = parser.add_subparsers(dest="command")

    p_list = subparsers.add_parser("list-sites", help="List all websites")
    p_list.add_argument("--name", help="Filter by name")

    p_create = subparsers.add_parser("create-proxy", help="Create custom-port reverse proxy")
    p_create.add_argument("--alias", required=True, help="Site alias name (e.g. drive)")
    p_create.add_argument("--port", required=True, type=int, help="1Panel custom listening port (e.g. 30004)")
    p_create.add_argument("--proxy-target", required=True, help="Internal target URL (e.g. http://192.168.8.11:8666)")
    p_create.add_argument("--primary-domain", help="Primary domain name")
    p_create.add_argument("--ssl-id", type=int, default=0, help="Existing SSL certificate ID")
    p_create.add_argument("--no-ssl", action="store_true", help="Disable SSL")

    p_ssl = subparsers.add_parser("apply-ssl", help="Apply ACME SSL certificate via DNS")
    p_ssl.add_argument("--domain", required=True, help="Primary domain (e.g. drive.335356119.xyz)")
    p_ssl.add_argument("--dns-id", type=int, default=1, help="DNS Account ID (default 1: Cloudflare)")
    p_ssl.add_argument("--acme-id", type=int, default=2, help="ACME Account ID (default 2)")

    p_certs = subparsers.add_parser("list-certs", help="List all SSL certificates")

    args = parser.parse_args()
    if args.command == "list-sites":
        cmd_list_sites(args)
    elif args.command == "create-proxy":
        cmd_create_proxy(args)
    elif args.command == "apply-ssl":
        cmd_apply_ssl(args)
    elif args.command == "list-certs":
        cmd_list_certs(args)
    else:
        parser.print_help()

if __name__ == "__main__":
    main()
