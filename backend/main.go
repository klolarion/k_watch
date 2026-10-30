package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/klolarion/k_watch/backend/handler"
)

func main() {
	r := gin.New()
	// Kubelet probes hit /api/healthz every few seconds, so keep them out of the access log.
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/api/healthz"}}), gin.Recovery())

	r.GET("/api/healthz", handler.Healthz)
	r.GET("/api/namespaces", handler.ListNamespaces)
	r.GET("/api/namespaces/:namespace/map", handler.NamespaceMap)
	r.GET("/api/namespaces/:namespace/flow", handler.NamespaceFlow)
	r.GET("/api/namespaces/:namespace/workloads/:kind/:name", handler.WorkloadFocus)
	r.GET("/api/cluster/overview", handler.ClusterOverview)

	r.Static("/assets", "./static/assets")
	r.StaticFile("/", "./static/index.html")
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if file, ok := staticFile(c.Request.URL.Path); ok {
			c.File(file)
			return
		}
		// Anything else is a client-side route, so the SPA handles it.
		c.File("./static/index.html")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8282"
	}
	if _, err := strconv.Atoi(port); err != nil {
		println("Invalid PORT environment variable, using default port 8282")
		port = "8282"
	}

	// Kubernetes sends SIGTERM before it stops the Pod. Let open requests finish and end
	// Pod streams, so the process exits 0 instead of being killed by the signal.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{Addr: ":" + port, Handler: r}
	srv.RegisterOnShutdown(handler.StopStreams)
	go func() {
		println("K-Watch backend starting on port " + port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			println("K-Watch backend stopped: " + err.Error())
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()
	println("K-Watch backend shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		println("K-Watch backend shutdown: " + err.Error())
	}
}

// staticFile maps a request path to a regular file under ./static, such as the favicon.
// path.Clean on a rooted path drops "..", so the result cannot leave ./static.
func staticFile(requestPath string) (string, bool) {
	file := filepath.Join("./static", filepath.FromSlash(path.Clean("/"+requestPath)))
	info, err := os.Stat(file)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return file, true
}
