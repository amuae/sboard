package geoip

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// GeoIP 数据库下载地址 (使用 Loyalsoldier 维护的版本，每日更新)
const (
	GeoIPURL = "https://ghfast.top/https://github.com/Loyalsoldier/geoip/releases/latest/download/Country.mmdb"
	// 备用地址
	GeoIPURLBackup = "https://cdn.jsdelivr.net/gh/Loyalsoldier/geoip@release/Country.mmdb"
)

var (
	db       *maxminddb.Reader
	dbMu     sync.RWMutex
	dbPath   string
	stopChan chan struct{}
)

// Record GeoIP 记录
type Record struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// Init 初始化 GeoIP 数据库
func Init(path string) error {
	dbPath = path
	stopChan = make(chan struct{})

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 尝试加载现有数据库
	if err := loadDB(); err != nil {
		log.Printf("GeoIP 数据库不存在或已损坏，正在下载...")
		if err := Download(); err != nil {
			return fmt.Errorf("下载 GeoIP 数据库失败: %v", err)
		}
		if err := loadDB(); err != nil {
			return err
		}
	}

	// 启动每日更新任务
	go startDailyUpdate()

	return nil
}

// loadDB 加载数据库文件
func loadDB() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	// 关闭旧的数据库
	if db != nil {
		db.Close()
		db = nil
	}

	reader, err := maxminddb.Open(dbPath)
	if err != nil {
		return err
	}

	db = reader
	return nil
}

// Close 关闭数据库
func Close() {
	if stopChan != nil {
		close(stopChan)
	}

	dbMu.Lock()
	defer dbMu.Unlock()

	if db != nil {
		db.Close()
		db = nil
	}
}

// Download 从 GitHub 下载最新的 GeoIP 数据库
func Download() error {
	log.Println("正在下载 GeoIP 数据库...")

	// 尝试主地址
	err := downloadFile(GeoIPURL, dbPath)
	if err != nil {
		log.Printf("主地址下载失败: %v，尝试备用地址...", err)
		// 尝试备用地址
		err = downloadFile(GeoIPURLBackup, dbPath)
		if err != nil {
			return fmt.Errorf("下载失败: %v", err)
		}
	}

	log.Println("GeoIP 数据库下载完成")
	return nil
}

// downloadFile 下载文件
func downloadFile(url, dest string) error {
	// 创建 HTTP 客户端，设置超时
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 写入临时文件
	tmpFile := dest + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tmpFile)
		return err
	}

	// 验证文件是否有效
	testReader, err := maxminddb.Open(tmpFile)
	if err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("文件验证失败: %v", err)
	}
	testReader.Close()

	// 重命名为正式文件
	if err := os.Rename(tmpFile, dest); err != nil {
		os.Remove(tmpFile)
		return err
	}

	return nil
}

// startDailyUpdate 启动每日更新任务
func startDailyUpdate() {
	// 计算到明天凌晨 3 点的时间
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 3, 0, 0, 0, now.Location())
	timer := time.NewTimer(next.Sub(now))

	for {
		select {
		case <-timer.C:
			log.Println("开始每日 GeoIP 数据库更新...")
			if err := Download(); err != nil {
				log.Printf("GeoIP 数据库更新失败: %v", err)
			} else {
				if err := loadDB(); err != nil {
					log.Printf("重新加载 GeoIP 数据库失败: %v", err)
				} else {
					log.Println("GeoIP 数据库更新成功")
				}
			}
			// 设置下一次更新时间（24小时后）
			timer.Reset(24 * time.Hour)

		case <-stopChan:
			timer.Stop()
			return
		}
	}
}

// LookupCountry 查询 IP 所属国家
func LookupCountry(ipStr string) string {
	dbMu.RLock()
	defer dbMu.RUnlock()

	if db == nil {
		return ""
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	var record Record
	err := db.Lookup(ip, &record)
	if err != nil {
		return ""
	}

	return record.Country.ISOCode
}

// IsChinaIP 判断是否为中国 IP
func IsChinaIP(ipStr string) bool {
	country := LookupCountry(ipStr)
	return country == "CN"
}

// IsLoaded 检查数据库是否已加载
func IsLoaded() bool {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return db != nil
}
