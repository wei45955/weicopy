package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/weicopy/backend/controllers"
	"github.com/weicopy/backend/middlewares"
	"github.com/weicopy/backend/models"
)

func main() {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using default environment variables")
	}

	// 设置运行模式
	gin.SetMode(getEnv("GIN_MODE", "debug"))

	// 初始化数据库
	models.ConnectDatabase()

	// 创建Gin实例
	r := gin.Default()

	// 配置CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// 设置静态文件目录
	r.Static("/uploads", "./uploads")

	// 路由组
	api := r.Group("/api")
	{
		// 认证路由
		auth := api.Group("/auth")
		{
			auth.POST("/register", controllers.Register)
			auth.POST("/login", controllers.Login)
			auth.GET("/me", middlewares.AuthRequired(), controllers.GetCurrentUser)
		}

		// 剪贴板路由 - 需要认证
		clipboard := api.Group("/clipboard").Use(middlewares.AuthRequired())
		{
			clipboard.GET("/", controllers.GetClipboardItems)
			clipboard.GET("/latest", controllers.GetLatestClipboardItem)
			clipboard.POST("/text", controllers.AddTextItem)
			clipboard.POST("/file", controllers.UploadFile)
			clipboard.POST("/image", controllers.UploadImage)
			clipboard.GET("/file/:id", controllers.GetFile)
			clipboard.DELETE("/:id", controllers.DeleteClipboardItem)
		}
	}

	// 启动服务器
	port := getEnv("PORT", "8080")
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
