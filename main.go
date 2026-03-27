package main

import (
	"fmt"
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
	Date        string         `gorm:"index" json:"date"`       // 任务所属日期 (用于每日重置逻辑)
	CompletedAt *time.Time     `json:"completed_at"`            // 完成时间戳 (指针类型，未完成时为 nil)
	CreatedAt   time.Time      `json:"created_at"`
}

// --- 2. 全局变量与初始化 ---

var db *gorm.DB

func InitDB() {
	var err error
	// 使用 sqlite 存储数据
	db, err = gorm.Open(sqlite.Open("daytodo.db"), &gorm.Config{})
	if err != nil {
		panic("❌ 数据库连接失败")
	}
	
	// 自动迁移表结构 (会自动添加新字段)
	db.AutoMigrate(&Task{})
	fmt.Println("✅ DayTodo 数据库已就绪")
}

// --- 3. 前端页面 (HTML/CSS/JS) ---

// 使用 const 存储 HTML 模板，方便单文件运行
const indexHTML = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DayTodo - 每日清单</title>
    <style>
        :root { --primary: #4a90e2; --success: #2ecc71; --danger: #e74c3c; --bg: #f5f7fa; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: var(--bg); color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        h1 { text-align: center; color: #2c3e50; margin-bottom: 5px; }
        .subtitle { text-align: center; color: #7f8c8d; font-size: 0.9em; margin-bottom: 20px; }
        
        .card { background: #fff; padding: 20px; border-radius: 12px; box-shadow: 0 4px 15px rgba(0,0,0,0.05); }
        
        /* 输入区域 */
        .input-group { display: flex; gap: 10px; margin-bottom: 25px; }
        input[type="text"] { flex: 1; padding: 12px; border: 2px solid #eee; border-radius: 8px; outline: none; transition: 0.3s; }
        input[type="text"]:focus { border-color: var(--primary); }
        .btn-add { padding: 12px 25px; background: var(--primary); color: white; border: none; border-radius: 8px; cursor: pointer; font-weight: bold; }
        .btn-add:hover { background: #357abd; }

        /* 任务列表 */
        ul { list-style: none; padding: 0; margin: 0; }
        li { display: flex; align-items: center; padding: 12px 0; border-bottom: 1px solid #f0f0f0; animation: fadeIn 0.3s ease; }
        li:last-child { border-bottom: none; }
        
        .task-content { flex: 1; margin-left: 12px; }
        .task-title { font-size: 1.1em; display: block; }
        .task-meta { font-size: 0.85em; color: #888; margin-top: 4px; display: block; }
        .completed-time { color: var(--success); font-weight: bold; }

        /* 按钮 */
        .actions { display: flex; gap: 8px; }
        .btn { padding: 6px 12px; border: none; border-radius: 6px; cursor: pointer; font-size: 0.9em; transition: 0.2s; }
        .btn-check { background: #e8f5e9; color: var(--success); }
        .btn-check:hover { background: var(--success); color: white; }
        .btn-del { background: #ffebee; color: var(--danger); }
        .btn-del:hover { background: var(--danger); color: white; }

        /* 完成状态样式 */
        .is-done .task-title { text-decoration: line-through; color: #bdc3c7; }
        .is-done .btn-check { background: #bdc3c7; color: white; cursor: default; }
        
        /* 历史查询 */
        .history-box { margin-top: 30px; padding-top: 20px; border-top: 2px solid #eee; }
        .history-header { display: flex; gap: 10px; margin-bottom: 15px; }
        .history-item { padding: 10px; background: #fafafa; border-radius: 6px; margin-bottom: 8px; display: flex; justify-content: space-between; }

        @keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }
    </style>
</head>
<body>
    <h1>DayTodo</h1>
    <div class="subtitle">📅 {{ .Today }}</div>

    <div class="card">
        <!-- 添加任务 -->
        <div class="input-group">
            <input type="text" id="titleInput" placeholder="今天要做些什么？" onkeypress="if(event.key==='Enter') addTask()">
            <button class="btn-add" onclick="addTask()">添加</button>
        </div>

        <!-- 今日列表 -->
        <ul id="taskList">
            {{ range .Tasks }}
            <li class="{{ if .IsDone }}is-done{{ end }}">
                <div class="task-content">
                    <span class="task-title">{{ .Title }}</span>
                    {{ if .IsDone }}
                        <span class="task-meta">完成于: <span class="completed-time">{{ .CompletedAt.Format "15:04:05" }}</span></span>
                    {{ else }}
                        <span class="task-meta">待处理</span>
                    {{ end }}
                </div>
                <div class="actions">
                    <button class="btn btn-check" onclick="toggleTask({{ .ID }})">{{ if .IsDone }}已完{{ else }}完成{{ end }}</button>
                    <button class="btn btn-del" onclick="deleteTask({{ .ID }})">删除</button>
                </div>
            </li>
            {{ end }}
        </ul>

        <!-- 历史查询 -->
        <div class="history-box">
            <h3>🔍 历史复盘</h3>
            <div class="history-header">
                <input type="date" id="historyDate">
                <button class="btn-add" style="padding: 10px 20px;" onclick="queryHistory()">查询</button>
            </div>
            <div id="historyResult"></div>
        </div>
    </div>

    <script>
        // 默认日期设为今天
        document.getElementById('historyDate').value = new Date().toISOString().split('T')[0];

        async function addTask() {
            const input = document.getElementById('titleInput');
            const title = input.value.trim();
            if (!title) return;

            await fetch('/api/tasks', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({ title: title, date: new Date().toISOString().split('T')[0] })
            });
            input.value = '';
            location.reload();
        }

        async function toggleTask(id) {
            await fetch(`/api/tasks/${id}/toggle`, { method: 'POST' });
            location.reload();
        }

        async function deleteTask(id) {
            if(!confirm('确定删除吗？')) return;
            await fetch(`/api/tasks/${id}`, { method: 'DELETE' });
            location.reload();
        }

        async function queryHistory() {
            const date = document.getElementById('historyDate').value;
            const res = await fetch(`/api/history?date=${date}`);
            const data = await res.json();
            
            const container = document.getElementById('historyResult');
            if (data.length === 0) {
                container.innerHTML = '<p style="text-align:center; color:#999;">该日期无记录</p>';
                return;
            }

            let html = '';
            data.forEach(task => {
                const statusColor = task.is_done ? '#2ecc71' : '#e74c3c';
                const statusText = task.is_done ? '✅ 已完成' : '⏳ 未完成';
                const timeText = task.completed_at ? ` (${task.completed_at})` : '';
                
                html += `
                <div class="history-item">
                    <span style="text-decoration: ${task.is_done ? 'line-through' : 'none'}; color: ${task.is_done ? '#888' : '#333'}">${task.title}</span>
                    <span style="color: ${statusColor}; font-size: 0.85em;">${statusText}${timeText}</span>
                </div>`;
            });
            container.innerHTML = html;
        }
    </script>
</body>
</html>
`

// --- 4. 路由与业务逻辑 ---

func main() {
	InitDB()
	r := gin.Default()

	// 加载 HTML 模板
	r.LoadHTMLFromReader(strings.NewReader(indexHTML), "index.html")

	// 首页
	r.GET("/", func(c *gin.Context) {
		today := time.Now().Format("2006-01-02")
		var tasks []Task
		// 仅查询今日数据，按 ID 倒序排列
		db.Where("date = ?", today).Order("id DESC").Find(&tasks)

		c.HTML(http.StatusOK, "index.html", gin.H{
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

	// API: 切换状态 (核心逻辑：记录时间戳)
	r.POST("/api/tasks/:id/toggle", func(c *gin.Context) {
		id := c.Param("id")
		var task Task
		
		// 1. 查找任务
		if err := db.First(&task, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
			return
		}

		// 2. 翻转状态
		task.IsDone = !task.IsDone

		// 3. 处理时间戳
		if task.IsDone {
			// 如果标记为完成，记录当前时间
			now := time.Now()
			task.CompletedAt = &now
		} else {
			// 如果撤销完成，清空时间
			task.CompletedAt = nil
		}

		// 4. 保存
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
		// 查询指定日期的所有任务
		db.Where("date = ?", date).Order("id DESC").Find(&tasks)
		c.JSON(http.StatusOK, tasks)
	})

	fmt.Println("🚀 DayTodo 服务已启动: http://localhost:8080")
	r.Run(":8080")
}

// 引入 strings 包
import "strings"
