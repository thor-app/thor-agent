package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

type Metrics struct {
	TenantId    string  `json:"tenantId"`
	Key         string  `json:"key"`
	Timestamp   string  `json:"timestamp"`
	CPUUsage    float64 `json:"cpuUsagePercent"`
	MemoryUsed  uint64  `json:"memoryUsedMb"`
	MemoryTotal uint64  `json:"memoryTotalMb"`
	DiskUsed    uint64  `json:"diskUsedGb"`
	DiskTotal   uint64  `json:"diskTotalGb"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println(".env 파일 로드를 실패하였습니다. :", err)
		return
	}

	key := os.Getenv("SERVER_API_KEY")

	if key == "" {
		fmt.Println(".env 파일 내 SERVER_API_KEY가 존재하지 않습니다. :", err)
		return
	}

	header := http.Header{}
	header.Set("X-Agent-Key", key)

	url := "ws://localhost:8080/api/v1/public/monitoring"
	conn, response, err := websocket.DefaultDialer.Dial(url, header)

	tenantId := response.Header.Get("X-TENANT-ID")

	if tenantId == "" {
		fmt.Println("테넌트가 존재하지 않음", err)
		return
	}

	if err != nil {
		fmt.Println("웹 소켓 연결 실패", err)
		return
	}

	defer conn.Close()
	for {
		metrics, err := collectMetrics(tenantId, key)
		if err != nil {
			log.Printf("메트릭스 수집 실패: %v", err)
			continue
		}

		err = conn.WriteMessage(websocket.TextMessage, metrics)
		if err != nil {
			log.Println("전송 오류:", err)
			continue
		}

		log.Println("전송 완료:", string(metrics))
		time.Sleep(5 * time.Second)
	}
}

func collectMetrics(tenantId string, key string) ([]byte, error) {
	// CPU 사용률
	cpuPercent, err := cpu.Percent(1*time.Second, false)
	if err != nil {
		return []byte{}, err
	}

	cpuPercent[0] = math.Round(cpuPercent[0]*10) / 10

	// Memory 사용량
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return []byte{}, err
	}

	// 루트 기준 Disk 사용량
	diskStats, err := disk.Usage("/")
	if err != nil {
		return []byte{}, err
	}

	metrics := Metrics{
		TenantId:    tenantId,
		Key:         key,
		Timestamp:   time.Now().Format(time.RFC3339),
		CPUUsage:    cpuPercent[0],
		MemoryUsed:  memStats.Used / 1024 / 1024,          // MB 단위
		MemoryTotal: memStats.Total / 1024 / 1024,         // MB 단위
		DiskUsed:    diskStats.Used / 1024 / 1024 / 1024,  // GB 단위
		DiskTotal:   diskStats.Total / 1024 / 1024 / 1024, // GB 단위
	}

	metricsJSON, err := json.Marshal(metrics)
	if err != nil {
		return []byte{}, err
	}

	return metricsJSON, nil
}
