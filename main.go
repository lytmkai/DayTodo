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

// Task 任务模板表（存放固定的任务清单）
type Task struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Title     string `json:"title"`
	IsDeleted bool   `gorm:"default:false" json:"is_deleted"` // 软删除标记
}

// Record 每日记录表（存放每天的实际完成情况）
type Record struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	TaskID      uint       `json:"task_id"`
	Task        Task       `gorm:"foreignKey:TaskID" json:"task"` // 关联任务
	Date        string     `gorm:"index" json:"date"`             // 日期索引
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
	// 自动迁移两个表
	db.AutoMigrate(&Task{}, &Record{})
	fmt.Println("✅ DayTodo 数据库已就绪")
}

// --- 核心逻辑：每日自动重置/填充 ---
func ensureDailyRecords(today string) {
	// 1. 检查今天是否已经有记录了
	var count int64
	db.Model(&Record{}).Where("date = ?", today).Count(&count)

	// 2. 如果今天没有记录，说明是新的一天，需要从 Task 表同步任务
	if count == 0 {
		fmt.Println("📅 检测到新的一天，正在生成今日待办...")
		var tasks []Task
		// 获取所有未被软删除的任务
		db.Where("is_deleted = ?", false).Find(&tasks)

		// 批量插入到 Record 表
		for _, t := range tasks {
			db.Create(&Record{
				TaskID: t.ID,
				Date:   today,
				IsDone: false, // 每天都是新的开始，状态重置为 false
			})
		}
	}
}

// --- 4. 路由与业务逻辑 ---

func main() {
	gin.SetMode(gin.ReleaseMode)
	InitDB()
	r := gin.Default()

	// 加载嵌入的 HTML 模板
	tmpl := template.Must(template.New("index").Parse(indexHTML))
	r.SetHTMLTemplate(tmpl)

	// --- 页面路由 ---

	// 首页
	r.GET("/", func(c *gin.Context) {
		today := time.Now().Format("2006-01-02")
		
		// 1. 确保今天的数据已生成
		ensureDailyRecords(today)

		// 2. 查询今天的记录（预加载关联的 Task 信息）
		var records []Record
		db.Preload("Task").Where("date = ?", today).Order("id ASC").Find(&records)

		c.HTML(http.StatusOK, "index", gin.H{
			"Today":   today,
			"Records": records,
		})
	})

	// --- API 路由 ---

	// 1. 添加新任务模板 (添加到清单中)
	r.POST("/api/tasks", func(c *gin.Context) {
		var input struct {
			Title string `json:"title"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}
		db.Create(&Task{Title: input.Title})
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// 2. 标记任务完成/取消 (操作 Record 表)
	r.POST("/api/records/:id/toggle", func(c *gin.Context) {
		id := c.Param("id")
		var record Record

		if err := db.First(&record, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
			return
		}

		record.IsDone = !record.IsDone

		if record.IsDone {
			now := time.Now()
			record.CompletedAt = &now
		} else {
			record.CompletedAt = nil
		}

		db.Save(&record)
		c.JSON(http.StatusOK, gin.H{"status": "updated", "completed_at": record.CompletedAt})
	})

	// 3. 软删除任务模板 (操作 Task 表)
	r.DELETE("/api/tasks/:id", func(c *gin.Context) {
		id := c.Param("id")
		// 只是标记删除，不真正从数据库抹除
		db.Model(&Task{}).Where("id = ?", id).Update("is_deleted", true)
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	})

	// 4. 查询历史
	r.GET("/api/history", func(c *gin.Context) {
		date := c.Query("date")
		if date == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "日期不能为空"})
			return
		}
		var records []Record
		db.Preload("Task").Where("date = ?", date).Order("id ASC").Find(&records)
		c.JSON(http.StatusOK, records)
	})

	fmt.Println("🚀 DayTodo 服务已启动: http://localhost:8080")
	r.Run(":8080")
}
