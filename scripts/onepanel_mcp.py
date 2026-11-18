#!/usr/bin/env python3
"""
1Panel Native MCP Server for Ashan FRP Project
Interacts with 1Panel v2 API using HMAC/MD5 auth signature.
Supports websites, custom-port reverse proxies, SSL certificates, containers, monitoring, etc.
"""

import os
import sys
import json
import time
import hashlib
import urllib.request
import urllib.parse
import urllib.error
from typing import Dict, Any, List, Optional
from mcp.server.mcpserver import MCPServer

server = MCPServer("1panel")

def _load_env(path: str = "/opt/data/.env") -> Dict[str, str]:
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

def _get_client_config():
    env = _load_env()
    base_url = env.get("ONEPANEL_BASE_URL", "http://192.168.8.11:10086/").rstrip("/")
    api_key = env.get("ONEPANEL_API_KEY", "")
    if not api_key:
        raise ValueError("Missing ONEPANEL_API_KEY in .env")
    return base_url, api_key

def _request(method: str, path: str, body: Optional[Dict[str, Any]] = None, query: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
    base_url, api_key = _get_client_config()
    
    # 1Panel v2 requires HMAC timestamp + md5 signature:
    # token = md5("1panel" + api_key + timestamp)
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

@server.tool()
def list_websites(page: int = 1, page_size: int = 50, name: str = "") -> str:
    """List websites managed by 1Panel including reverse proxies and status."""
    res = _request("POST", "/api/v2/websites/search", body={
        "page": page,
        "pageSize": page_size,
        "name": name,
        "orderBy": "created_at",
        "order": "descending"
    })
    return json.dumps(res, ensure_ascii=False, indent=2)

@server.tool()
def get_website_detail(website_id: int) -> str:
    """Get full configuration details of a website in 1Panel by website ID."""
    res = _request("GET", f"/api/v2/websites/{website_id}")
    return json.dumps(res, ensure_ascii=False, indent=2)

@server.tool()
def get_website_https(website_id: int) -> str:
    """Get HTTPS and TLS certificate binding details for a website."""
    res = _request("GET", f"/api/v2/websites/{website_id}/https")
    return json.dumps(res, ensure_ascii=False, indent=2)

@server.tool()
def create_custom_port_proxy_website(
    alias: str,
    proxy_target: str,
    port: int,
    primary_domain: str = "",
    enable_ssl: bool = True,
    ssl_id: int = 0,
    remark: str = ""
) -> str:
    """Create a dedicated custom-port reverse-proxy website in 1Panel (e.g. 30004 -> 8666) for ChmlFrp coordination."""
    payload = {
        "primaryDomain": primary_domain or alias,
        "alias": alias,
        "type": "proxy",
        "appType": "installed",
        "webSiteGroupID": 1,
        "proxy": proxy_target,
        "proxyType": "",
        "domains": [
            {
                "domain": "192.168.8.11",
                "port": port
            }
        ],
        "enableSSL": enable_ssl,
        "websiteSSLID": ssl_id,
        "remark": remark or f"{port} {alias}"
    }
    res = _request("POST", "/api/v2/websites", body=payload)
    return json.dumps(res, ensure_ascii=False, indent=2)

@server.tool()
def delete_website(website_id: int) -> str:
    """Delete a website in 1Panel v2."""
    payload = {
        "id": website_id,
        "deleteApp": False,
        "deleteBackup": False,
        "deleteDB": False,
        "forceDelete": True
    }
    res = _request("POST", "/api/v2/websites/del", body=payload)
    return json.dumps(res, ensure_ascii=False, indent=2)

@server.tool()
def list_certificates(page: int = 1, page_size: int = 50, name: str = "") -> str:
    """List SSL certificates in 1Panel with expiration, domains, and status."""
    res = _request("POST", "/api/v2/websites/ssl/search", body={
        "page": page,
        "pageSize": page_size,
        "name": name
    })
    return json.dumps(res, ensure_ascii=False, indent=2)

@server.tool()
def get_certificate_detail(ssl_id: int) -> str:
    """Get detailed information about an SSL certificate in 1Panel."""
    res = _request("GET", f"/api/v2/websites/ssl/{ssl_id}")
    return json.dumps(res, ensure_ascii=False, indent=2)

@server.tool()
def apply_ssl_certificate(
    primary_domain: str,
    dns_account_id: int = 1,
    acme_account_id: int = 2,
    key_type: str = "EC256",
    auto_renew: bool = True
) -> str:
    """Apply for and issue an ACME SSL certificate via DNS challenge (Cloudflare, etc.)."""
    payload = {
        "primaryDomain": primary_domain,
        "provider": "dnsAccount",
        "acmeAccountId": acme_account_id,
        "dnsAccountId": dns_account_id,
        "autoRenew": auto_renew,
        "keyType": key_type,
        "apply": True
    }
    res = _request("POST", "/api/v2/websites/ssl", body=payload)
    return json.dumps(res, ensure_ascii=False, indent=2)

@server.tool()
def get_dashboard_metrics() -> str:
    """Get 1Panel host system status, CPU, memory, disk, and load metrics."""
    res = _request("GET", "/api/v2/dashboard/base/os")
    return json.dumps(res, ensure_ascii=False, indent=2)

@server.tool()
def get_mcp_domain() -> str:
    """Get 1Panel internal MCP server domain binding and status."""
    res = _request("GET", "/api/v2/ai/mcp/domain/get")
    return json.dumps(res, ensure_ascii=False, indent=2)

@server.tool()
def search_mcp_servers() -> str:
    """Search registered MCP servers within 1Panel."""
    res = _request("POST", "/api/v2/ai/mcp/search", body={"page": 1, "pageSize": 20})
    return json.dumps(res, ensure_ascii=False, indent=2)

if __name__ == "__main__":
    server.run()
