package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/weicopy/backend/config"
	"github.com/weicopy/backend/controllers"
	"github.com/weicopy/backend/middlewares"
	"github.com/weicopy/backend/models"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file, using environment variables")
	}
	gin.SetMode(getEnv("GIN_MODE", gin.DebugMode))

	if err := models.ConnectDatabase(); err != nil {
		log.Fatal(err)
	}

	index := filepath.Join(config.GetStaticDir(), "index.html")
	if _, err := os.Stat(index); err != nil {
		log.Fatalf("frontend not found: %s", index)
	}

	models.StartPurger()
	port := getEnv("PORT", "8080")
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("WeiCopy listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func newRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	if gin.Mode() != gin.TestMode {
		router.Use(gin.Logger())
	}
	router.Use(securityHeaders())
	router.Use(cors.New(cors.Config{
		AllowOrigins:  []string{"*"},
		AllowMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders: []string{"Content-Length", "Content-Disposition"},
	}))
	router.MaxMultipartMemory = 8 << 20

	api := router.Group("/api")
	api.Use(noStore())
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		api.GET("/sessions/status", controllers.SessionStatus)
		api.POST("/sessions", controllers.CreateSession)

		session := api.Group("/sessions/:name")
		session.Use(middlewares.SessionAccess())
		session.GET("/state", controllers.GetState)
		session.DELETE("", controllers.DeleteSession)
		session.POST("/items/text", controllers.AddText)
		session.POST("/items/file", controllers.Upload)
		session.GET("/items/latest", controllers.GetLatest)
		session.PUT("/items/:id", controllers.UpdateText)
		session.DELETE("/items/:id", controllers.DeleteItem)
		session.GET("/items/:id/file", controllers.GetFile)
	}

	mountStatic(router)
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "接口不存在"})
			return
		}
		c.Status(http.StatusNotFound)
	})
	return router
}

func mountStatic(router *gin.Engine) {
	dir := config.GetStaticDir()
	index := filepath.Join(dir, "index.html")
	if _, err := os.Stat(index); err != nil {
		return
	}
	router.StaticFile("/static/app.js", filepath.Join(dir, "app.js"))
	router.StaticFile("/static/styles.css", filepath.Join(dir, "styles.css"))
	router.StaticFile("/static/favicon.svg", filepath.Join(dir, "favicon.svg"))
	serveIndex := func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.File(index)
	}
	router.GET("/", serveIndex)
	router.HEAD("/", serveIndex)
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; img-src 'self' blob:; style-src 'self'; script-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		c.Next()
	}
}

func noStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
