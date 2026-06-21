package main

import (
	"embed"
	"[github.com/gin-gonic/gin](https://github.com/gin-gonic/gin)"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

//go:embed dist/*
var frontendAssets embed.FS

func SetupFRPRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// 1. 零开销静态资产内嵌
	subFS, err := fs.Sub(frontendAssets, "dist")
	if err == nil {
		r.StaticFS("/ui", http.FS(subFS))
	}

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/ui/")
	})

	api := r.Group("/api")
	{
		api.GET("/version", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"version":   "2.0.0-frp-go",
				"engine":    "pure-go-embedded",
				"status":    "healthy",
			})
		})

		api.GET("/tunnels", func(c *gin.Context) {
			var list []FRPTunnel
			DB.Order("id desc").Find(&list)
			c.JSON(http.StatusOK, gin.H{
				"tunnels": list,
			})
		})

		api.GET("/chmlfrp/nodes", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"nodes": []gin.H{
					{"id": 1, "name": "徐州电信高防 [双线]", "status": "online", "load": 32, "location": "中国江苏"},
					{"id": 2, "name": "香港 BGP 极速 [国际]", "status": "online", "load": 18, "location": "中国香港"},
				},
			})
		})

		api.GET("/frpc/runtime/info", func(c *gin.Context) {
			Manager.mu.Lock()
			cnt := len(Manager.running)
			Manager.mu.Unlock()

			c.JSON(http.StatusOK, gin.H{
				"frpc_version":         "0.54.0-native",
				"active_tunnels_count": cnt,
				"engine_status":        "running",
			})
		})
	}
	return r
}
