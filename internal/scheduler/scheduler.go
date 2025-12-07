package scheduler

import (
	"log"
	"time"

	"github.com/sboard-go/sboard/internal/database"
)

// ConfigSyncCallback 配置同步回调函数类型
type ConfigSyncCallback func()

// Scheduler 定时任务调度器
type Scheduler struct {
	stopChan       chan struct{}
	onConfigChange ConfigSyncCallback
	lastResetMonth int // 上次流量重置的月份
}

// New 创建新的调度器
func New() *Scheduler {
	return &Scheduler{
		stopChan:       make(chan struct{}),
		lastResetMonth: int(time.Now().Month()) - 1, // 初始化为上个月，确保启动时检查
	}
}

// SetConfigSyncCallback 设置配置同步回调
func (s *Scheduler) SetConfigSyncCallback(callback ConfigSyncCallback) {
	s.onConfigChange = callback
}

// Start 启动调度器
func (s *Scheduler) Start() {
	log.Println("定时任务调度器已启动")

	// 启动时先执行一次检查
	go s.checkExpiredUsers()
	go s.checkMonthlyTrafficReset()

	// 每10分钟检查一次到期用户
	go s.runPeriodically(10*time.Minute, s.checkExpiredUsers)
	// 每10分钟检查一次是否需要重置月度流量
	go s.runPeriodically(10*time.Minute, s.checkMonthlyTrafficReset)
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	close(s.stopChan)
	log.Println("定时任务调度器已停止")
}

// runPeriodically 定期执行任务
func (s *Scheduler) runPeriodically(interval time.Duration, task func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			task()
		case <-s.stopChan:
			return
		}
	}
}

// checkExpiredUsers 检查并禁用到期用户
func (s *Scheduler) checkExpiredUsers() {
	today := time.Now().Format("2006-01-02")

	// 查找所有已启用但已到期的用户
	var expiredUsers []database.ProxyUser
	result := database.DB.Where("enabled = ? AND expiry_date < ?", 1, today).Find(&expiredUsers)

	if result.Error != nil {
		log.Printf("检查到期用户失败: %v", result.Error)
		return
	}

	if len(expiredUsers) == 0 {
		return
	}

	// 禁用到期用户
	disabledCount := 0
	for _, user := range expiredUsers {
		user.Enabled = 0
		if err := database.DB.Save(&user).Error; err != nil {
			log.Printf("禁用用户 %s 失败: %v", user.Name, err)
			continue
		}
		disabledCount++
		log.Printf("用户 %s 已到期，已自动禁用", user.Name)
	}

	if disabledCount > 0 {
		log.Printf("共禁用 %d 个到期用户，触发配置同步", disabledCount)
		// 触发配置同步回调
		if s.onConfigChange != nil {
			go s.onConfigChange()
		}
	}
}

// checkMonthlyTrafficReset 检查并重置月度流量
func (s *Scheduler) checkMonthlyTrafficReset() {
	currentMonth := int(time.Now().Month())

	// 如果是新的一个月，重置所有服务器的月度流量
	if currentMonth != s.lastResetMonth {
		now := time.Now()
		result := database.DB.Model(&database.Server{}).Where("1 = 1").Updates(map[string]interface{}{
			"monthly_in":    0,
			"monthly_out":   0,
			"traffic_reset": now,
		})

		if result.Error != nil {
			log.Printf("重置月度流量失败: %v", result.Error)
			return
		}

		s.lastResetMonth = currentMonth
		log.Printf("已重置所有服务器的月度流量 (共 %d 台)", result.RowsAffected)
	}
}

// CheckExpiredUsersNow 立即检查到期用户（供 API 调用）
func CheckExpiredUsersNow() (int, error) {
	today := time.Now().Format("2006-01-02")

	// 查找并禁用所有已到期的用户
	result := database.DB.Model(&database.ProxyUser{}).
		Where("enabled = ? AND expiry_date < ?", 1, today).
		Update("enabled", 0)

	if result.Error != nil {
		return 0, result.Error
	}

	return int(result.RowsAffected), nil
}

// GetExpiredUsers 获取即将到期的用户列表
func GetExpiredUsers(days int) ([]database.ProxyUser, error) {
	futureDate := time.Now().AddDate(0, 0, days).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")

	var users []database.ProxyUser
	err := database.DB.Where("enabled = ? AND expiry_date >= ? AND expiry_date <= ?",
		1, today, futureDate).Find(&users).Error

	return users, err
}
