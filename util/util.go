package util

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/KforG/p2pool-scanner-go/logging"
)

// client times out so one unresponsive node can't stall a scan.
var client = &http.Client{Timeout: 10 * time.Second}

func GetJson(url string, target interface{}) error {
	return GetJsonCtx(context.Background(), url, target)
}

// GetJsonCtx fetches http://url and decodes the JSON answer into target,
// giving up when ctx ends.
func GetJsonCtx(ctx context.Context, url string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s", url), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %s", url, resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(target)
	if err != nil {
		logging.Errorf("Error parsing response from %s\n", url)
		return err
	}
	return err
}

func RemoveStringSliceIndex(i int, s *[]string) {
	if i != len(*s)-1 {
		(*s)[i] = (*s)[len(*s)-1]
	}
	// drop the last element
	*s = (*s)[:len(*s)-1]
}
