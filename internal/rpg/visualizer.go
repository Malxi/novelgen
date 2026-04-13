package rpg

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Visualizer 可视化报告生成器
type Visualizer struct {
	data   *NovelRPGData
	report *SimulationReport
}

// NewVisualizer 创建可视化器
func NewVisualizer(data *NovelRPGData, report *SimulationReport) *Visualizer {
	return &Visualizer{
		data:   data,
		report: report,
	}
}

// GenerateHTMLReport 生成HTML可视化报告
func (v *Visualizer) GenerateHTMLReport(outputPath string) error {
	tmpl := `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>RPG模拟分析报告</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"></script>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        
        .container {
            max-width: 1400px;
            margin: 0 auto;
        }
        
        .header {
            text-align: center;
            color: white;
            margin-bottom: 30px;
        }
        
        .header h1 {
            font-size: 2.5em;
            margin-bottom: 10px;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.3);
        }
        
        .header .subtitle {
            font-size: 1.1em;
            opacity: 0.9;
        }
        
        .dashboard {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        
        .card {
            background: white;
            border-radius: 16px;
            padding: 24px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.1);
            transition: transform 0.3s ease, box-shadow 0.3s ease;
        }
        
        .card:hover {
            transform: translateY(-5px);
            box-shadow: 0 15px 50px rgba(0,0,0,0.15);
        }
        
        .card-title {
            font-size: 1.2em;
            font-weight: 600;
            color: #333;
            margin-bottom: 16px;
            display: flex;
            align-items: center;
            gap: 8px;
        }
        
        .card-icon {
            font-size: 1.5em;
        }
        
        .stat-grid {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 12px;
        }
        
        .stat-item {
            text-align: center;
            padding: 12px;
            background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
            border-radius: 12px;
        }
        
        .stat-value {
            font-size: 2em;
            font-weight: bold;
            color: #667eea;
        }
        
        .stat-label {
            font-size: 0.9em;
            color: #666;
            margin-top: 4px;
        }
        
        .score-circle {
            width: 150px;
            height: 150px;
            margin: 0 auto;
            position: relative;
        }
        
        .score-circle svg {
            transform: rotate(-90deg);
        }
        
        .score-circle-bg {
            fill: none;
            stroke: #e0e0e0;
            stroke-width: 10;
        }
        
        .score-circle-progress {
            fill: none;
            stroke: url(#scoreGradient);
            stroke-width: 10;
            stroke-linecap: round;
            transition: stroke-dasharray 1s ease;
        }
        
        .score-text {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            text-align: center;
        }
        
        .score-number {
            font-size: 2.5em;
            font-weight: bold;
            color: #667eea;
        }
        
        .score-grade {
            font-size: 1.2em;
            color: #666;
        }
        
        .validation-list {
            list-style: none;
        }
        
        .validation-item {
            display: flex;
            align-items: center;
            padding: 12px;
            margin-bottom: 8px;
            border-radius: 8px;
            background: #f8f9fa;
        }
        
        .validation-item.passed {
            background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%);
        }
        
        .validation-item.failed {
            background: linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%);
        }
        
        .validation-icon {
            font-size: 1.5em;
            margin-right: 12px;
        }
        
        .validation-info {
            flex: 1;
        }
        
        .validation-name {
            font-weight: 600;
            color: #333;
        }
        
        .validation-score {
            font-size: 0.9em;
            color: #666;
        }
        
        .character-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
            gap: 16px;
        }
        
        .character-card {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 16px;
            border-radius: 12px;
            text-align: center;
        }
        
        .character-name {
            font-size: 1.2em;
            font-weight: bold;
            margin-bottom: 8px;
        }
        
        .character-type {
            font-size: 0.9em;
            opacity: 0.9;
            padding: 4px 12px;
            background: rgba(255,255,255,0.2);
            border-radius: 20px;
            display: inline-block;
        }
        
        .timeline {
            position: relative;
            padding-left: 30px;
        }
        
        .timeline::before {
            content: '';
            position: absolute;
            left: 10px;
            top: 0;
            bottom: 0;
            width: 2px;
            background: linear-gradient(180deg, #667eea 0%, #764ba2 100%);
        }
        
        .timeline-item {
            position: relative;
            margin-bottom: 20px;
            padding: 16px;
            background: #f8f9fa;
            border-radius: 12px;
        }
        
        .timeline-item::before {
            content: '';
            position: absolute;
            left: -24px;
            top: 20px;
            width: 12px;
            height: 12px;
            background: #667eea;
            border-radius: 50%;
            border: 3px solid white;
            box-shadow: 0 0 0 3px #667eea;
        }
        
        .timeline-day {
            font-weight: bold;
            color: #667eea;
            margin-bottom: 8px;
        }
        
        .timeline-events {
            font-size: 0.9em;
            color: #666;
        }
        
        .issue-card {
            background: linear-gradient(135deg, #ff6b6b 0%, #ee5a24 100%);
            color: white;
            padding: 16px;
            border-radius: 12px;
            margin-bottom: 12px;
        }
        
        .issue-title {
            font-weight: bold;
            margin-bottom: 8px;
            display: flex;
            align-items: center;
            gap: 8px;
        }
        
        .issue-description {
            font-size: 0.95em;
            opacity: 0.95;
        }
        
        .issue-suggestion {
            margin-top: 8px;
            padding-top: 8px;
            border-top: 1px solid rgba(255,255,255,0.3);
            font-size: 0.9em;
        }
        
        .chart-container {
            position: relative;
            height: 300px;
            margin-top: 20px;
        }
        
        .mermaid {
            background: white;
            padding: 20px;
            border-radius: 12px;
        }
        
        .section-title {
            font-size: 1.8em;
            font-weight: bold;
            color: white;
            margin: 40px 0 20px;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.3);
        }
        
        .footer {
            text-align: center;
            color: white;
            margin-top: 40px;
            padding: 20px;
            opacity: 0.8;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎮 RPG模拟分析报告</h1>
            <div class="subtitle">生成时间: {{.GeneratedTime}}</div>
        </div>
        
        <!-- 总体评分 -->
        <div class="dashboard">
            <div class="card">
                <div class="card-title">
                    <span class="card-icon">🏆</span>
                    总体评分
                </div>
                <div class="score-circle">
                    <svg width="150" height="150">
                        <defs>
                            <linearGradient id="scoreGradient" x1="0%" y1="0%" x2="100%" y2="0%">
                                <stop offset="0%" style="stop-color:#667eea"/>
                                <stop offset="100%" style="stop-color:#764ba2"/>
                            </linearGradient>
                        </defs>
                        <circle class="score-circle-bg" cx="75" cy="75" r="65"/>
                        <circle class="score-circle-progress" cx="75" cy="75" r="65"
                                stroke-dasharray="{{.ScoreCircumference}}"
                                stroke-dashoffset="{{.ScoreOffset}}"/>
                    </svg>
                    <div class="score-text">
                        <div class="score-number">{{.Report.Summary.OverallScore}}</div>
                        <div class="score-grade">{{.Report.Summary.Grade}}</div>
                    </div>
                </div>
            </div>
            
            <div class="card">
                <div class="card-title">
                    <span class="card-icon">📊</span>
                    数据统计
                </div>
                <div class="stat-grid">
                    <div class="stat-item">
                        <div class="stat-value">{{.Report.Summary.TotalEvents}}</div>
                        <div class="stat-label">模拟事件</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">{{.Report.Summary.CharactersInvolved}}</div>
                        <div class="stat-label">涉及角色</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">{{.Report.Summary.IssuesFound}}</div>
                        <div class="stat-label">发现问题</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">{{.Report.Summary.CriticalIssues}}</div>
                        <div class="stat-label">严重问题</div>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- RPG数据概览 -->
        <div class="section-title">📦 RPG数据概览</div>
        <div class="dashboard">
            <div class="card">
                <div class="card-title">
                    <span class="card-icon">👥</span>
                    角色 ({{len .Data.Characters}})
                </div>
                <div class="character-grid">
                    {{range .Data.Characters}}
                    <div class="character-card">
                        <div class="character-name">{{.Name}}</div>
                        <div class="character-type">{{.Type}}</div>
                    </div>
                    {{end}}
                </div>
            </div>
            
            <div class="card">
                <div class="card-title">
                    <span class="card-icon">📍</span>
                    地点 ({{len .Data.Locations}})
                </div>
                <div class="character-grid">
                    {{range .Data.Locations}}
                    <div class="character-card" style="background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);">
                        <div class="character-name">{{.Name}}</div>
                        <div class="character-type">{{.Type}}</div>
                    </div>
                    {{end}}
                </div>
            </div>
        </div>
        
        <!-- 验证结果 -->
        <div class="section-title">✅ 验证结果</div>
        <div class="card">
            <ul class="validation-list">
                {{range .Report.ValidationResults}}
                <li class="validation-item {{if .Passed}}passed{{else}}failed{{end}}">
                    <span class="validation-icon">{{if .Passed}}✅{{else}}❌{{end}}</span>
                    <div class="validation-info">
                        <div class="validation-name">{{.Category}}</div>
                        <div class="validation-score">得分: {{.Score}}</div>
                    </div>
                    {{if .Issues}}
                    <div style="margin-left: auto; color: #e74c3c;">
                        {{len .Issues}} 个问题
                    </div>
                    {{end}}
                </li>
                {{end}}
            </ul>
        </div>
        
        <!-- 验证详情图表 -->
        <div class="card" style="margin-top: 20px;">
            <div class="card-title">
                <span class="card-icon">📈</span>
                验证评分图表
            </div>
            <div class="chart-container">
                <canvas id="validationChart"></canvas>
            </div>
        </div>
        
        <!-- 时间线 -->
        {{if .Data.Timeline}}
        <div class="section-title">📅 故事时间线</div>
        <div class="card">
            <div class="timeline">
                {{range .Data.Timeline}}
                <div class="timeline-item">
                    <div class="timeline-day">第 {{.Day}} 天</div>
                    <div class="timeline-events">
                        {{range .Events}}
                        <div>• {{.}}</div>
                        {{end}}
                    </div>
                    {{if .Location}}
                    <div style="margin-top: 8px; font-size: 0.85em; color: #999;">
                        📍 {{.Location}}
                    </div>
                    {{end}}
                </div>
                {{end}}
            </div>
        </div>
        {{end}}
        
        <!-- 问题与建议 -->
        {{if .Report.Recommendations}}
        <div class="section-title">⚠️ 问题与建议</div>
        <div class="dashboard">
            {{range .Report.Recommendations}}
            <div class="issue-card">
                <div class="issue-title">
                    <span>⚠️</span>
                    改进建议
                </div>
                <div class="issue-description">{{.}}</div>
            </div>
            {{end}}
        </div>
        {{end}}
        
        <!-- 验证问题详情 -->
        {{range .Report.ValidationResults}}
        {{if .Issues}}
        <div class="card" style="margin-top: 20px;">
            <div class="card-title">
                <span class="card-icon">🔍</span>
                {{.Category}} - 问题详情
            </div>
            {{range .Issues}}
            <div class="issue-card" style="background: linear-gradient(135deg, #ff9a9e 0%, #fecfef 100%); color: #333;">
                <div class="issue-description">{{.}}</div>
            </div>
            {{end}}
            {{if .Suggestions}}
            <div style="margin-top: 16px;">
                <strong>💡 建议:</strong>
                <ul style="margin-top: 8px; padding-left: 20px;">
                    {{range .Suggestions}}
                    <li>{{.}}</li>
                    {{end}}
                </ul>
            </div>
            {{end}}
        </div>
        {{end}}
        {{end}}
        
        <div class="footer">
            <p>由 NovelGen RPG模拟系统生成</p>
            <p style="font-size: 0.9em; margin-top: 8px;">使用 AI 提取 + RPG 模拟验证技术</p>
        </div>
    </div>
    
    <script>
        // 验证评分雷达图
        const ctx = document.getElementById('validationChart').getContext('2d');
        new Chart(ctx, {
            type: 'radar',
            data: {
                labels: {{.ChartLabels}},
                datasets: [{
                    label: '验证得分',
                    data: {{.ChartData}},
                    backgroundColor: 'rgba(102, 126, 234, 0.2)',
                    borderColor: 'rgba(102, 126, 234, 1)',
                    borderWidth: 2,
                    pointBackgroundColor: 'rgba(102, 126, 234, 1)',
                    pointBorderColor: '#fff',
                    pointHoverBackgroundColor: '#fff',
                    pointHoverBorderColor: 'rgba(102, 126, 234, 1)'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    r: {
                        beginAtZero: true,
                        max: 100,
                        ticks: {
                            stepSize: 20
                        }
                    }
                },
                plugins: {
                    legend: {
                        display: false
                    }
                }
            }
        });
        
        // 初始化 Mermaid
        mermaid.initialize({ startOnLoad: true });
    </script>
</body>
</html>
`

	type TemplateData struct {
		Data              *NovelRPGData
		Report            *SimulationReport
		GeneratedTime     string
		ScoreCircumference float64
		ScoreOffset       float64
		ChartLabels       template.JS
		ChartData         template.JS
	}

	// 计算评分圆环
	score := v.report.Summary.OverallScore
	circumference := 2 * 3.14159 * 65
	offset := circumference * (1 - score/100)

	// 准备图表数据
	var labels []string
	var data []float64
	for _, result := range v.report.ValidationResults {
		labels = append(labels, result.Category)
		data = append(data, result.Score)
	}

	labelsJSON, _ := json.Marshal(labels)
	dataJSON, _ := json.Marshal(data)

	templateData := TemplateData{
		Data:               v.data,
		Report:             v.report,
		GeneratedTime:      time.Now().Format("2006-01-02 15:04:05"),
		ScoreCircumference: circumference,
		ScoreOffset:        offset,
		ChartLabels:        template.JS(labelsJSON),
		ChartData:          template.JS(dataJSON),
	}

	// 创建输出目录
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 解析并执行模板
	t := template.Must(template.New("report").Parse(tmpl))
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	if err := t.Execute(file, templateData); err != nil {
		return fmt.Errorf("执行模板失败: %w", err)
	}

	return nil
}

// GenerateMarkdownReport 生成Markdown报告
func (v *Visualizer) GenerateMarkdownReport(outputPath string) error {
	var sb strings.Builder

	// 标题
	sb.WriteString("# 🎮 RPG模拟分析报告\n\n")
	sb.WriteString(fmt.Sprintf("生成时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// 总体评分
	sb.WriteString("## 🏆 总体评分\n\n")
	sb.WriteString(fmt.Sprintf("**评分: %.1f/100**\n\n", v.report.Summary.OverallScore))
	sb.WriteString(fmt.Sprintf("**评级: %s**\n\n", v.report.Summary.Grade))

	// 数据统计
	sb.WriteString("## 📊 数据统计\n\n")
	sb.WriteString(fmt.Sprintf("- 模拟事件: %d\n", v.report.Summary.TotalEvents))
	sb.WriteString(fmt.Sprintf("- 涉及角色: %d\n", v.report.Summary.CharactersInvolved))
	sb.WriteString(fmt.Sprintf("- 发现问题: %d\n", v.report.Summary.IssuesFound))
	sb.WriteString(fmt.Sprintf("- 严重问题: %d\n\n", v.report.Summary.CriticalIssues))

	// RPG数据
	sb.WriteString("## 📦 RPG数据\n\n")
	sb.WriteString(fmt.Sprintf("### 角色 (%d)\n\n", len(v.data.Characters)))
	for _, char := range v.data.Characters {
		sb.WriteString(fmt.Sprintf("- **%s** - %s\n", char.Name, char.Type))
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("### 地点 (%d)\n\n", len(v.data.Locations)))
	for _, loc := range v.data.Locations {
		sb.WriteString(fmt.Sprintf("- **%s** - %s\n", loc.Name, loc.Type))
	}
	sb.WriteString("\n")

	// 验证结果
	sb.WriteString("## ✅ 验证结果\n\n")
	for _, result := range v.report.ValidationResults {
		status := "✅"
		if !result.Passed {
			status = "❌"
		}
		sb.WriteString(fmt.Sprintf("### %s %s (得分: %.1f)\n\n", status, result.Category, result.Score))
		
		if len(result.Issues) > 0 {
			sb.WriteString("**问题:**\n")
			for _, issue := range result.Issues {
				sb.WriteString(fmt.Sprintf("- %s\n", issue))
			}
			sb.WriteString("\n")
		}
		
		if len(result.Suggestions) > 0 {
			sb.WriteString("**建议:**\n")
			for _, suggestion := range result.Suggestions {
				sb.WriteString(fmt.Sprintf("- %s\n", suggestion))
			}
			sb.WriteString("\n")
		}
	}

	// 时间线
	if len(v.data.Timeline) > 0 {
		sb.WriteString("## 📅 故事时间线\n\n")
		for _, tl := range v.data.Timeline {
			sb.WriteString(fmt.Sprintf("### 第 %d 天\n\n", tl.Day))
			for _, event := range tl.Events {
				sb.WriteString(fmt.Sprintf("- %s\n", event))
			}
			if tl.Location != "" {
				sb.WriteString(fmt.Sprintf("\n📍 地点: %s\n", tl.Location))
			}
			sb.WriteString("\n")
		}
	}

	// 建议
	if len(v.report.Recommendations) > 0 {
		sb.WriteString("## 💡 改进建议\n\n")
		for i, rec := range v.report.Recommendations {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec))
		}
		sb.WriteString("\n")
	}

	// 写入文件
	return os.WriteFile(outputPath, []byte(sb.String()), 0644)
}

// GenerateJSONReport 生成JSON报告
func (v *Visualizer) GenerateJSONReport(outputPath string) error {
	report := map[string]interface{}{
		"generated_time": time.Now().Format("2006-01-02 15:04:05"),
		"summary":        v.report.Summary,
		"data": map[string]interface{}{
			"characters": len(v.data.Characters),
			"items":      len(v.data.Items),
			"skills":     len(v.data.Skills),
			"locations":  len(v.data.Locations),
			"events":     len(v.data.Events),
			"timeline":   len(v.data.Timeline),
		},
		"validation_results": v.report.ValidationResults,
		"recommendations":    v.report.Recommendations,
	}

	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	return os.WriteFile(outputPath, jsonData, 0644)
}

// OpenBrowser 尝试在浏览器中打开报告
func (v *Visualizer) OpenBrowser(reportPath string) error {
	// 获取绝对路径
	absPath, err := filepath.Abs(reportPath)
	if err != nil {
		return err
	}

	// 转换为 file:// URL
	url := "file://" + absPath

	// 根据操作系统打开浏览器
	var cmd string
	var args []string

	switch os := os.Getenv("GOOS"); os {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	// 使用系统命令打开浏览器
	return exec.Command(cmd, args...).Start()
}

// 简化函数：生成所有格式的报告
func GenerateAllReports(data *NovelRPGData, report *SimulationReport, outputDir string) ([]string, error) {
	viz := NewVisualizer(data, report)
	
	var files []string
	
	// HTML报告
	htmlPath := filepath.Join(outputDir, "rpg_report.html")
	if err := viz.GenerateHTMLReport(htmlPath); err != nil {
		return nil, fmt.Errorf("生成HTML报告失败: %w", err)
	}
	files = append(files, htmlPath)
	
	// Markdown报告
	mdPath := filepath.Join(outputDir, "rpg_report.md")
	if err := viz.GenerateMarkdownReport(mdPath); err != nil {
		return nil, fmt.Errorf("生成Markdown报告失败: %w", err)
	}
	files = append(files, mdPath)
	
	// JSON报告
	jsonPath := filepath.Join(outputDir, "rpg_report.json")
	if err := viz.GenerateJSONReport(jsonPath); err != nil {
		return nil, fmt.Errorf("生成JSON报告失败: %w", err)
	}
	files = append(files, jsonPath)
	
	return files, nil
}
