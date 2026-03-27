package main

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// --- 1. 数据库模型 ---

type Task struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Title       string         `json:"title"`
	IsDone      bool           `json:"is_done"`
	Date        string         `gorm:"index" json:"date"`
	CompletedAt *time.Time     `json:"completed_at"`
	CreatedAt   time.Time      `json:"created_at"`
}

// --- 2. 静态资源嵌入 ---

//go:embed index.html
var indexHTML string

// --- 3. 全局变量与初始化 ---

var db *gorm.DB

func InitDB() {
	var err error
	db, err = gorm.Open(sqlite.Open("daytodo.db"), &gorm.Config{})
	if err != nil {
		panic("❌ 数据库连接失败")
	}
	db.AutoMigrate(&Task{})
	fmt.Println("✅ DayTodo 数据库已就绪")
}

// --- 4. 路由与业务逻辑 ---

func main() {
	InitDB()
	r := gin.Default()

	// 加载嵌入的 HTML 模板
	tmpl := template.Must(template.New("index").Parse(indexHTML))
	r.SetHTMLTemplate(tmpl)

	// 首页
	r.GET("/", func(c *gin.Context) {
		today := time.Now().Format("2006-01-02")
		var tasks []Task
		db.Where("date = ?", today).Order("id DESC").Find(&tasks)

		c.HTML(http.StatusOK, "index", gin.H{
			"Today": today,
			"Tasks": tasks,
		})
	})

	// API: 添加任务
	r.POST("/api/tasks", func(c *gin.Context) {
		var input struct {
			Title string `json:"title"`
			Date  string `json:"date"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}
		db.Create(&Task{Title: input.Title, Date: input.Date, IsDone: false})
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// API: 切换状态
	r.POST("/api/tasks/:id/toggle", func(c *gin.Context) {
		id := c.Param("id")
		var task Task

		if err := db.First(&task, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
			return
		}

		task.IsDone = !task.IsDone

		if task.IsDone {
			now := time.Now()
			task.CompletedAt = &now
		} else {
			task.CompletedAt = nil
		}

		db.Save(&task)
		c.JSON(http.StatusOK, gin.H{"status": "updated", "completed_at": task.CompletedAt})
	})

	// API: 删除任务
	r.DELETE("/api/tasks/:id", func(c *gin.Context) {
		db.Delete(&Task{}, c.Param("id"))
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	})

	// API: 查询历史
	r.GET("/api/history", func(c *gin.Context) {
		date := c.Query("date")
		if date == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "日期不能为空"})
			return
		}
		var tasks []Task
		db.Where("date = ?", date).Order("id DESC").Find(&tasks)
		c.JSON(http.StatusOK, tasks)
	})

	fmt.Println("🚀 DayTodo 服务已启动: http://localhost:8080")
	r.Run(":8080")
}
