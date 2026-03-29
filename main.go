package main

import (
	_ "embed"
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
	ID        uint   `gorm:"primaryKey" json:"id"`
	Title     string `gorm:"uniqueIndex;not null" json:"title"`
	IsDeleted bool   `gorm:"default:false" json:"is_deleted"`
}

type Record struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	TaskID      uint       `json:"task_id"`
	Task        Task       `gorm:"foreignKey:TaskID" json:"task"`
	Date        string     `gorm:"index" json:"date"`
	IsDone      bool       `json:"is_done"`
	CompletedAt *time.Time `json:"completed_at"`
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
	db.AutoMigrate(&Task{}, &Record{})
	fmt.Println("✅ DayTodo 数据库已就绪")
}

// --- 4. 路由与业务逻辑 ---

func main() {
	gin.SetMode(gin.ReleaseMode)
	InitDB()
	r := gin.Default()

	tmpl := template.Must(template.New("index").Parse(indexHTML))
	r.SetHTMLTemplate(tmpl)

	// 首页
	r.GET("/", func(c *gin.Context) {
		// 修改点：直接从 Task 表加载列表，不再依赖 Record 表
		var tasks []Task
		db.Where("is_deleted = ?", false).Order("id DESC").Find(&tasks)

		c.HTML(http.StatusOK, "index", gin.H{
			"Today": time.Now().Format("2006-01-02"),
			"Tasks": tasks, // 传递任务列表
		})
	})

	// 添加新任务
	r.POST("/api/tasks", func(c *gin.Context) {
		var input struct {
			Title string `json:"title"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}

		// 检查重复
		var existingTask Task
		if err := db.Where("title = ?", input.Title).First(&existingTask).Error; err == nil {
			if existingTask.IsDeleted {
				db.Model(&existingTask).Update("is_deleted", false)
				c.JSON(http.StatusOK, gin.H{"status": "restored"})
			} else {
				c.JSON(http.StatusConflict, gin.H{"error": "同名任务已存在"})
			}
			return
		}

		db.Create(&Task{Title: input.Title})
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// 标记完成 (核心逻辑修改点)
	r.POST("/api/tasks/:id/complete", func(c *gin.Context) {
		id := c.Param("id")
		var task Task

		if err := db.First(&task, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
			return
		}

		// 修改点：点击完成时，才向 Record 表插入一条记录
		now := time.Now()
		record := Record{
			TaskID:      task.ID,
			Date:        now.Format("2006-01-02"), // 记录今天的日期
			IsDone:      true,
			CompletedAt: &now,
		}
		db.Create(&record)

		c.JSON(http.StatusOK, gin.H{"status": "completed", "time": now.Format("15:04:05")})
	})

	// 软删除
	r.DELETE("/api/tasks/:id", func(c *gin.Context) {
		db.Model(&Task{}).Where("id = ?", c.Param("id")).Update("is_deleted", true)
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	})

	// 查询历史
	r.GET("/api/history", func(c *gin.Context) {
		date := c.Query("date")
		if date == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "日期不能为空"})
			return
		}
		// 查询指定日期的完成记录
		var records []Record
		db.Preload("Task").Where("date = ?", date).Order("id ASC").Find(&records)
		c.JSON(http.StatusOK, records)
	})

	fmt.Println("🚀 DayTodo 服务已启动: http://localhost:8080")
	r.Run(":8080")
}
