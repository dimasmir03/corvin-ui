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
	ServerRepo *repository.ServerRepo
}

func NewCollectTotalOnlineJob(repo *repository.ServerRepo) *CollectOnlineJob {
	return &CollectOnlineJob{ServerRepo: repo}
}

type serverOnlineResult struct {
	Server models.Server
	Count  int
	Err    error
}

func (j *CollectOnlineJob) Run() {
	servers, err := j.ServerRepo.GetAll()
	if err != nil {
		fmt.Printf("[ERROR] failed to get servers: %v\n", err)
		return
	}

	var (
		wg      sync.WaitGroup
		results = make(chan serverOnlineResult, len(servers))
	)

	for _, server := range servers {
		wg.Add(1)
		go func(s models.Server) {
			defer wg.Done()

			count, err := j.fetchOnlineCount(&s)
			results <- serverOnlineResult{s, count, err}
		}(server)
	}

	wg.Wait()
	close(results)

	totalOnline := 0

	for res := range results {
		if res.Err != nil {
			fmt.Printf("[WARN] %s: failed to get online users: %v\n", res.Server.Name, res.Err)
			continue
		}

		totalOnline += res.Count

		stat := models.ServerStat{
			ServerID: res.Server.Id,
			Online:   res.Count,
		}

		if err := j.ServerRepo.CreateStat(&stat); err != nil {
			fmt.Printf("[ERROR] failed to save stat for %s: %v\n", res.Server.Name, err)
		}
	}

	totalstat := models.ServerStat{
		ServerID: 0,
		Online:   totalOnline,
	}

	if err := j.ServerRepo.CreateStat(&totalstat); err != nil {
		fmt.Printf("[ERROR] failed to save total online stat: %v\n", err)
	}

}

func (j *CollectOnlineJob) fetchOnlineCount(server *models.Server) (int, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	url := fmt.Sprintf(
		"http://%s:%d%spanel/api/inbounds/onlines",
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
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var response struct {
		Success bool     `json:"success"`
		Msg     string   `json:"msg"`
		Obj     []string `json:"obj"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0, fmt.Errorf("failed to decode json: %w", err)
	}

	if !response.Success {
		return 0, fmt.Errorf("api error: %s", response.Msg)
	}

	return len(response.Obj), nil
}
