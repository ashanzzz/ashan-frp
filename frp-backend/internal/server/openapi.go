package server

var openAPISpec = []byte(`{
  "openapi": "3.0.3",
  "info": {
    "title": "Ashan FRP API",
    "version": "0.1.0",
    "description": "Go rewrite control plane for Ashan FRP"
  },
  "servers": [{"url": "/"}],
  "paths": {
    "/api/v1/version": {"get": {"summary": "Version"}},
    "/api/v1/health": {"get": {"summary": "Health"}},
    "/api/v1/nodes": {"get": {"summary": "List nodes"}, "post": {"summary": "Create node"}},
    "/api/v1/nodes/{id}": {"get": {"summary": "Get node"}, "patch": {"summary": "Update node"}},
    "/api/v1/tunnels": {"get": {"summary": "List tunnels"}, "post": {"summary": "Create tunnel"}},
    "/api/v1/tunnels/{id}": {"get": {"summary": "Get tunnel"}, "patch": {"summary": "Update tunnel"}},
    "/api/v1/website-mappings": {"get": {"summary": "List website mappings"}, "post": {"summary": "Create website mapping"}},
    "/api/v1/settings": {"get": {"summary": "Get settings"}, "patch": {"summary": "Update settings"}},
    "/api/v1/jobs": {"get": {"summary": "List jobs"}},
    "/api/v1/jobs/{id}": {"get": {"summary": "Get job"}},
    "/api/v1/events/stream": {"get": {"summary": "SSE stream"}},
    "/api/v1/frpc/runtime": {"get": {"summary": "Runtime status"}}
  }
}`)
