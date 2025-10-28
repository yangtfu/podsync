package builder

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// API URL常量
const (
	// 剧集信息API
	// https://api.bilibili.com/x/web-interface/view?bvid=BV1KDskz1EHD
	EpisodeInfoAPI = "https://api.bilibili.com/x/web-interface/view?bvid=%s"

	// 用户信息API
	UserInfoAPI = "https://api.bilibili.com/x/web-interface/card?mid=%s"

	// 用户剧集列表API ps max 100, ps=0显示全部
	// https://api.bilibili.com/x/series/recArchivesByKeywords?mid=1596926736&keywords&ps=0
	UserEpisodesAPI = "https://api.bilibili.com/x/series/recArchivesByKeywords?keywords=&mid=%s&pn=%d&ps=%d"

	// Season类型播放列表信息API ps max 100
	// https://api.bilibili.com/x/polymer/web-space/seasons_archives_list?season_id=678635&mid=7380321&page_num=1&page_size=100
	SeasonInfoAPI = "https://api.bilibili.com/x/polymer/web-space/seasons_archives_list?season_id=%s&mid=%s&page_num=%d&page_size=%d"

	// Series类型播放列表信息API
	// https://api.bilibili.com/x/series/series?series_id=1067956
	SeriesInfoAPI = "https://api.bilibili.com/x/series/series?series_id=%s"

	// Series类型播放列表剧集API ps=0显示全部
	// https://api.bilibili.com/x/series/archives?mid=7458285&series_id=1067956&ps=0&pn=1
	SeriesEpisodesAPI = "https://api.bilibili.com/x/series/archives?mid=%s&series_id=%s&ps=%d&pn=%d"

	MaxBilibiliPageSize = 100
)

// APIClient API客户端
type APIClient struct {
	client *http.Client
}

// NewAPIClient 创建新的API客户端
func NewAPIClient() *APIClient {
	return &APIClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// setRequestHeaders 设置请求头，参考 https://github.com/CuteReimu/bilibili/blob/master/client.go
func (c *APIClient) setRequestHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Origin", "https://www.bilibili.com")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0")
}

// DoRequest 发送API请求并解析响应
func (c *APIClient) DoRequest(url string, result any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	c.setRequestHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	return nil
}

// 剧集信息API响应结构体
type EpisodeResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Aid                 int    `json:"aid"`
		Bvid                string `json:"bvid"`
		Title               string `json:"title"`
		Desc                string `json:"desc"`
		Pic                 string `json:"pic"`
		PubDate             int64  `json:"pubdate"`
		Duration            int    `json:"duration"`
		Is_upower_exclusive bool   `json:"is_upower_exclusive"`
		Owner               struct {
			Mid  int    `json:"mid"`
			Name string `json:"name"`
			Face string `json:"face"`
		} `json:"owner"`
		Stat struct {
			View int `json:"view"`
		} `json:"stat"`
	} `json:"data"`
}

// GetEpisodeInfo 获取剧集信息
func (c *APIClient) GetEpisodeInfo(bvid string) (*EpisodeResponse, error) {
	apiURL := fmt.Sprintf(EpisodeInfoAPI, bvid)

	var response EpisodeResponse
	if err := c.DoRequest(apiURL, &response); err != nil {
		return nil, err
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", response.Message)
	}

	return &response, nil
}

// 用户信息API响应结构体
type UserResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Card struct {
			Mid       string `json:"mid"`
			Name      string `json:"name"`
			Face      string `json:"face"`
			Sign      string `json:"sign"`
			Fans      int    `json:"fans"`
			LevelInfo struct {
				CurrentLevel int `json:"current_level"`
			} `json:"level_info"`
			Official struct {
				Title string `json:"title"`
			} `json:"official"`
		} `json:"card"`
		Space struct {
			ViewCount int `json:"viewcount"`
		} `json:"space,omitempty"`
		Follower int `json:"follower"`
	} `json:"data"`
}

// GetUserInfo 获取用户信息
func (c *APIClient) GetUserInfo(mid string) (*UserResponse, error) {
	apiURL := fmt.Sprintf(UserInfoAPI, mid)

	var response UserResponse
	if err := c.DoRequest(apiURL, &response); err != nil {
		return nil, err
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", response.Message)
	}

	return &response, nil
}

type Archive struct {
	Aid      int64  `json:"aid"`
	Bvid     string `json:"bvid"`
	CTime    int64  `json:"ctime"`
	Duration int    `json:"duration"`
	Pic      string `json:"pic"`
	PubDate  int64  `json:"pubdate"`
	Stat     struct {
		View int `json:"view"`
	} `json:"stat"`
	State int    `json:"state"`
	Title string `json:"title"`
}

// 用户剧集列表API响应结构体
type UserEpisodesResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Archives []Archive `json:"archives"`
		Page     struct {
			Num   int `json:"num"`
			Size  int `json:"size"`
			Total int `json:"total"`
		} `json:"page"`
	} `json:"data"`
}

// GetUserEpisodesByPage 分页获取用户剧集列表
func (c *APIClient) GetUserEpisodesByPage(mid string, pageNum, pageSize int) (*UserEpisodesResponse, error) {
	if pageSize > MaxBilibiliPageSize || pageSize == 0 {
		pageSize = 0
		pageNum = 1
	}
	apiURL := fmt.Sprintf(UserEpisodesAPI, mid, pageNum, pageSize)

	var response UserEpisodesResponse
	if err := c.DoRequest(apiURL, &response); err != nil {
		return nil, err
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", response.Message)
	}

	return &response, nil
}

// Season类型播放列表API响应结构体
type SeasonArchivesResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	TTL     int    `json:"ttl"`
	Data    struct {
		Aids     []int64   `json:"aids"`
		Archives []Archive `json:"archives"`
		Meta     struct {
			Category    int    `json:"category"`
			Cover       string `json:"cover"`
			Description string `json:"description"`
			Mid         int    `json:"mid"`
			Name        string `json:"name"`
			PTime       int64  `json:"ptime"`
			SeasonID    int    `json:"season_id"`
			Total       int    `json:"total"`
		} `json:"meta"`
		Page struct {
			PageNum  int `json:"page_num"`
			PageSize int `json:"page_size"`
			Total    int `json:"total"`
		} `json:"page"`
	} `json:"data"`
}

// GetSeasonEpisodesByPage 分页获取Season类型播放列表剧集
func (c *APIClient) GetSeasonEpisodesByPage(mid, seasonID string, pageNum, pageSize int) (*SeasonArchivesResponse, error) {
	if pageSize > MaxBilibiliPageSize || pageSize == 0 {
		pageSize = MaxBilibiliPageSize
	}
	apiURL := fmt.Sprintf(SeasonInfoAPI, seasonID, mid, pageNum, pageSize)

	var response SeasonArchivesResponse
	if err := c.DoRequest(apiURL, &response); err != nil {
		return nil, err
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", response.Message)
	}

	return &response, nil
}

// Series类型播放列表API响应结构体
type SeriesInfoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	TTL     int    `json:"ttl"`
	Data    struct {
		Meta struct {
			SeriesID     int      `json:"series_id"`
			Mid          int      `json:"mid"`
			Name         string   `json:"name"`
			Description  string   `json:"description"`
			Keywords     []string `json:"keywords"`
			Creator      string   `json:"creator"`
			State        int      `json:"state"`
			LastUpdateTs int64    `json:"last_update_ts"`
			Total        int      `json:"total"`
			Ctime        int64    `json:"ctime"`
			Mtime        int64    `json:"mtime"`
			RawKeywords  string   `json:"raw_keywords"`
			Category     int      `json:"category"`
		} `json:"meta"`
		RecentAids []int64 `json:"recent_aids"`
	} `json:"data"`
}

// GetSeriesInfo 获取Series类型播放列表信息
func (c *APIClient) GetSeriesInfo(seriesID string) (*SeriesInfoResponse, error) {
	apiURL := fmt.Sprintf(SeriesInfoAPI, seriesID)

	var response SeriesInfoResponse
	if err := c.DoRequest(apiURL, &response); err != nil {
		return nil, err
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", response.Message)
	}

	return &response, nil
}

// Series类型播放列表剧集API响应结构体
type SeriesArchivesResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Archives []Archive `json:"archives"`
		Page     struct {
			Num   int `json:"num"`
			Size  int `json:"size"`
			Count int `json:"count"`
		} `json:"page"`
	} `json:"data"`
}

// GetSeriesEpisodesByPage 分页获取Series类型播放列表剧集
func (c *APIClient) GetSeriesEpisodesByPage(mid, seriesID string, pageNum, pageSize int) (*SeriesArchivesResponse, error) {
	if pageSize > MaxBilibiliPageSize || pageSize == 0 {
		pageSize = 0
		pageNum = 1
	}
	apiURL := fmt.Sprintf(SeriesEpisodesAPI, mid, seriesID, pageSize, pageNum)

	var response SeriesArchivesResponse
	if err := c.DoRequest(apiURL, &response); err != nil {
		return nil, err
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", response.Message)
	}

	return &response, nil
}
