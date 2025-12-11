package netstatic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	gnet "github.com/shirou/gopsutil/v3/net"
)

/*
统计每个网卡的流量情况，保存最近 DataPreserveDay 天的数据，每 DetectInterval 秒采集一次

默认保存到 Agent 工作目录下的 net_static.json 文件中
所有操作都尽可能在内存中完成，避免频繁的IO操作
只有在启动、停止和保存时，才会进行文件的读写操作
*/

const (
	DefaultDataPreserveDay = 31.0     // 保存最近多少天的数据
	DefaultDetectInterval  = 2.0      // 采集间隔(秒)
	DefaultSaveInterval    = 60.0 * 5 // 保存间隔(秒)，5分钟
	SaveFileName           = "net_static.json"
)

// NetStaticConfig 配置
type NetStaticConfig struct {
	DataPreserveDay float64  `json:"data_preserve_day"` // 保存最近多少天的数据
	DetectInterval  float64  `json:"detect_interval"`   // 采集间隔(秒)
	SaveInterval    float64  `json:"save_interval"`     // 保存间隔(秒)
	Nics            []string `json:"nics"`              // 仅监控指定的网卡名称列表，空表示监控所有
}

// TrafficData 流量数据（增量）
type TrafficData struct {
	Timestamp uint64 `json:"timestamp"`
	Tx        uint64 `json:"tx"` // 发送字节数增量
	Rx        uint64 `json:"rx"` // 接收字节数增量
}

// NetStatic 网卡流量统计数据
type NetStatic struct {
	Interfaces map[string][]TrafficData `json:"interfaces"` // key: 网卡名称
	Config     NetStaticConfig          `json:"config"`
}

var (
	mu           sync.RWMutex
	running      bool
	detectTicker *time.Ticker
	saveTicker   *time.Ticker
	stopCh       chan struct{}

	// 内存持久区
	store NetStatic

	// 缓存区（未保存到文件的数据）
	staticCache map[string][]TrafficData

	// 上次采集到的累计字节数（用于计算 delta）
	lastCounters = map[string]struct{ Tx, Rx uint64 }{}

	// 配置
	config   NetStaticConfig
	dataPath string // 数据文件路径
)

func nowUnix() uint64 { return uint64(time.Now().Unix()) }

// isNicAllowed 判断网卡是否在监控白名单内
func isNicAllowed(name string) bool {
	if len(config.Nics) == 0 {
		return true
	}
	for _, n := range config.Nics {
		if n == name {
			return true
		}
	}
	return false
}

func ensureInitLocked() {
	if store.Interfaces == nil {
		store.Interfaces = make(map[string][]TrafficData)
	}
	if staticCache == nil {
		staticCache = make(map[string][]TrafficData)
	}
	if config.DataPreserveDay == 0 {
		config.DataPreserveDay = DefaultDataPreserveDay
	}
	if config.DetectInterval == 0 {
		config.DetectInterval = DefaultDetectInterval
	}
	if config.SaveInterval == 0 {
		config.SaveInterval = DefaultSaveInterval
	}
}

func loadFromFileLocked() error {
	if dataPath == "" {
		return nil
	}
	data, err := os.ReadFile(dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var loaded NetStatic
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	store = loaded
	if store.Interfaces == nil {
		store.Interfaces = make(map[string][]TrafficData)
	}
	// 应用保存的配置
	if loaded.Config.DataPreserveDay > 0 {
		config.DataPreserveDay = loaded.Config.DataPreserveDay
	}
	return nil
}

func saveToFileLocked() error {
	if dataPath == "" {
		return nil
	}
	store.Config = config
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	// 确保目录存在
	dir := filepath.Dir(dataPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(dataPath, data, 0644)
}

// purgeExpiredLocked 清理过期数据
func purgeExpiredLocked() {
	ttl := time.Duration(config.DataPreserveDay * 24 * float64(time.Hour))
	cutoff := uint64(time.Now().Add(-ttl).Unix())
	for name, arr := range store.Interfaces {
		kept := arr[:0]
		for _, td := range arr {
			if td.Timestamp >= cutoff {
				kept = append(kept, td)
			}
		}
		if len(kept) == 0 {
			delete(store.Interfaces, name)
		} else {
			store.Interfaces[name] = kept
		}
	}
}

// safeDelta 安全计算差值，处理计数器回绕或重置
func safeDelta(cur, prev uint64) uint64 {
	if cur >= prev {
		return cur - prev
	}
	// 计数器回绕或重置，返回 0
	return 0
}

// 最近一次采样的时间和增量（用于计算实时网速）
var (
	lastSampleTime  time.Time
	lastSampleDelta = map[string]struct{ Tx, Rx uint64 }{}
)

// sampleOnceLocked 采集一次流量数据
func sampleOnceLocked() {
	ios, err := gnet.IOCounters(true)
	if err != nil {
		return
	}
	ts := nowUnix()
	now := time.Now()
	for _, io := range ios {
		name := io.Name
		if !isNicAllowed(name) {
			continue
		}
		curTx := io.BytesSent
		curRx := io.BytesRecv
		prev, ok := lastCounters[name]
		if ok {
			dtx := safeDelta(curTx, prev.Tx)
			drx := safeDelta(curRx, prev.Rx)
			// 保存本次增量（用于计算实时网速）
			lastSampleDelta[name] = struct{ Tx, Rx uint64 }{Tx: dtx, Rx: drx}
			// 首次采样不记录，有增量才记录
			if dtx > 0 || drx > 0 {
				staticCache[name] = append(staticCache[name], TrafficData{Timestamp: ts, Tx: dtx, Rx: drx})
			}
		}
		lastCounters[name] = struct{ Tx, Rx uint64 }{Tx: curTx, Rx: curRx}
	}
	lastSampleTime = now
}

// flushCacheLocked 将缓存数据合并到持久区
func flushCacheLocked(ts uint64) {
	if len(staticCache) == 0 {
		return
	}
	for name, arr := range staticCache {
		var sumTx, sumRx uint64
		for _, td := range arr {
			sumTx += td.Tx
			sumRx += td.Rx
		}
		if sumTx > 0 || sumRx > 0 {
			store.Interfaces[name] = append(store.Interfaces[name], TrafficData{Timestamp: ts, Tx: sumTx, Rx: sumRx})
		}
	}
	staticCache = make(map[string][]TrafficData)
}

// startGoroutinesLocked 启动采集和保存的 goroutines
func startGoroutinesLocked() {
	// 采集 goroutine
	go func() {
		for {
			select {
			case <-detectTicker.C:
				mu.Lock()
				sampleOnceLocked()
				mu.Unlock()
			case <-stopCh:
				return
			}
		}
	}()

	// 保存 goroutine
	go func() {
		for {
			select {
			case t := <-saveTicker.C:
				mu.Lock()
				flushCacheLocked(uint64(t.Unix()))
				purgeExpiredLocked()
				_ = saveToFileLocked()
				mu.Unlock()
			case <-stopCh:
				return
			}
		}
	}()
}

// Init 初始化流量统计模块
func Init(path string) error {
	mu.Lock()
	defer mu.Unlock()

	dataPath = path
	ensureInitLocked()

	// 加载历史数据
	if err := loadFromFileLocked(); err != nil {
		// 加载失败不影响启动
	}

	return nil
}

// Start 启动流量统计
func Start() error {
	mu.Lock()
	defer mu.Unlock()

	if running {
		return nil
	}

	ensureInitLocked()

	// 启动 ticker
	detectTicker = time.NewTicker(time.Duration(config.DetectInterval * float64(time.Second)))
	saveTicker = time.NewTicker(time.Duration(config.SaveInterval * float64(time.Second)))
	stopCh = make(chan struct{})
	running = true

	startGoroutinesLocked()
	return nil
}

// Stop 停止流量统计
func Stop() error {
	mu.Lock()
	defer mu.Unlock()

	if !running {
		return nil
	}
	running = false

	if detectTicker != nil {
		detectTicker.Stop()
	}
	if saveTicker != nil {
		saveTicker.Stop()
	}
	close(stopCh)

	// 最后一次 flush + 保存
	flushCacheLocked(nowUnix())
	purgeExpiredLocked()
	return saveToFileLocked()
}

// GetTotalTraffic 获取总流量统计数据
func GetTotalTraffic() (map[string]TrafficData, error) {
	mu.RLock()
	defer mu.RUnlock()

	res := map[string]TrafficData{}
	add := func(name string, tx, rx uint64) {
		cur := res[name]
		cur.Tx += tx
		cur.Rx += rx
		res[name] = cur
	}

	// 从持久区累加
	for name, arr := range store.Interfaces {
		var tx, rx uint64
		for _, td := range arr {
			tx += td.Tx
			rx += td.Rx
		}
		add(name, tx, rx)
	}

	// 从缓存区累加
	for name, arr := range staticCache {
		var tx, rx uint64
		for _, td := range arr {
			tx += td.Tx
			rx += td.Rx
		}
		add(name, tx, rx)
	}

	return res, nil
}

// GetTotalTrafficBetween 获取指定时间段内的总流量统计数据
func GetTotalTrafficBetween(start, end uint64) (map[string]TrafficData, error) {
	mu.RLock()
	defer mu.RUnlock()

	res := map[string]TrafficData{}
	inRange := func(ts uint64) bool {
		return (start == 0 || ts >= start) && (end == 0 || ts <= end)
	}
	add := func(name string, tx, rx uint64) {
		cur := res[name]
		cur.Tx += tx
		cur.Rx += rx
		res[name] = cur
	}

	// 从持久区累加
	for name, arr := range store.Interfaces {
		var tx, rx uint64
		for _, td := range arr {
			if inRange(td.Timestamp) {
				tx += td.Tx
				rx += td.Rx
			}
		}
		if tx > 0 || rx > 0 {
			add(name, tx, rx)
		}
	}

	// 从缓存区累加
	for name, arr := range staticCache {
		var tx, rx uint64
		for _, td := range arr {
			if inRange(td.Timestamp) {
				tx += td.Tx
				rx += td.Rx
			}
		}
		if tx > 0 || rx > 0 {
			add(name, tx, rx)
		}
	}

	return res, nil
}

// GetCurrentSpeed 获取当前实时网速（基于最近一次采样的增量）
// 返回的是 bytes/second
func GetCurrentSpeed() (upSpeed, downSpeed uint64, err error) {
	mu.RLock()
	defer mu.RUnlock()

	// 如果采样间隔过长（超过10秒），认为数据无效
	if time.Since(lastSampleTime) > 10*time.Second {
		return 0, 0, nil
	}

	// 计算采样间隔（秒）
	interval := config.DetectInterval
	if interval <= 0 {
		interval = DefaultDetectInterval
	}

	// 累加所有网卡的增量
	var totalUp, totalDown uint64
	for name, delta := range lastSampleDelta {
		if !ShouldIncludeNic(name) {
			continue
		}
		totalUp += delta.Tx
		totalDown += delta.Rx
	}

	// 换算成每秒的字节数
	upSpeed = uint64(float64(totalUp) / interval)
	downSpeed = uint64(float64(totalDown) / interval)
	return upSpeed, downSpeed, nil
}

// Clear 清除所有流量统计数据
func Clear() error {
	mu.Lock()
	defer mu.Unlock()

	store.Interfaces = make(map[string][]TrafficData)
	staticCache = make(map[string][]TrafficData)
	lastCounters = map[string]struct{ Tx, Rx uint64 }{}
	return nil
}
