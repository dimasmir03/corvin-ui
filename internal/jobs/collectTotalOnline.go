package jobs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
)

type CollectOnlineJob struct {
	ServerRepo repository.ServerRepo
}

func NewCollectTotalOnlineJob(repo repository.ServerRepo) *CollectOnlineJob {
	return &CollectOnlineJob{ServerRepo: repo}
}

type serverOnlineResult struct {
	Server models.Server
	Count  int
	Err    error
}

// Run запускает сбор онлайна со всех серверов
func (j *CollectOnlineJob) Run() {
	servers, err := j.ServerRepo.GetAll()
	if err != nil {
		fmt.Printf("[ERROR] failed to get servers: %v\n", err)
		return
	}

	// Параллельный сбор данных
	var wg sync.WaitGroup
	results := make(chan serverOnlineResult, len(servers))

	for _, server := range servers {
		wg.Add(1)
		go func(s models.Server) {
			defer wg.Done()
			count, err := getOnlineUsersServer(&s)
			results <- serverOnlineResult{s, count, err}
		}(server)
	}

	wg.Wait()
	close(results)

	totalOnline := 0

	// Обновляем данные в БД
	for res := range results {
		if res.Err != nil {
			fmt.Printf("[WARN] Failed to get online users from %s: %v\n", res.Server.Name, res.Err)
			continue
		}

		totalOnline += res.Count

		// // обновляем онлайн конкретного сервера
		// if err := j.ServerRepo.UpdateOnline(res.Server.Id, res.Count); err != nil {
		// 	fmt.Printf("[ERROR] update %s: %v\n", res.Server.Name, err)
		// }

		// сохраняем историю (опционально)
		stat := models.ServerStat{
			ServerID: res.Server.Id,
			Online:   res.Count,
		}

		if err := j.ServerRepo.CreateStat(&stat); err != nil {
			fmt.Printf("[ERROR] save %s stat: %v\n", res.Server.Name, err)
		}
	}

	// сохраняем общий онлайн (например, в Redis или в отдельной таблице)
	// if err := j.ServerRepo.SaveTotalOnline(totalOnline); err != nil {
	// 	fmt.Printf("[ERROR] failed to save total online: %v\n", err)
	// }
	totalstat := models.ServerStat{
		ServerID: 0,
		Online:   totalOnline,
	}
	if err := j.ServerRepo.CreateStat(&totalstat); err != nil {
		fmt.Printf("[ERROR] Failed to create total stat: %v\n", err)
	}

}

func getOnlineUsersServer(server *models.Server) (int, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	url := fmt.Sprintf("http://%s:%d%spanel/api/inbounds/onlines",
		server.IP,
		server.Port,
		server.SecretWebPath,
	)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-API-KEY", server.ApiKey)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	///////////////////////////
	// DEBUG BLOCK ////////////
	////////////////////////////
	// body, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	log.Printf("Failed to read response body: %v\n", err)
	// }
	// // req url
	// log.Println("Request URL:", req.URL.String())

	// // req header X-API-KEY
	// log.Println("Request Header X-API-KEY:", req.Header.Get("X-API-KEY"))

	// log.Println("Response status code:", resp.StatusCode)
	// // response body as string
	// log.Printf("Response body: %s\n", string(body))
	/////////////////////////////

	var onlineResponse struct {
		Success bool     `json:"success"`
		Msg     string   `json:"msg"`
		Obj     []string `json:"obj"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&onlineResponse); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if !onlineResponse.Success {
		return 0, fmt.Errorf("api responded with error: %s", onlineResponse.Msg)
	}

	return len(onlineResponse.Obj), nil
}
