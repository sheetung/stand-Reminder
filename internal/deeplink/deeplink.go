package deeplink

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	Scheme = "standreminder"
	Host   = "action"
)

func ActionURL(action string) string {
	return fmt.Sprintf("%s://%s?action=%s", Scheme, Host, url.QueryEscape(action))
}

func ParseActionArg(args []string) (string, bool) {
	for _, arg := range args {
		raw := strings.TrimSpace(arg)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if !strings.EqualFold(u.Scheme, Scheme) || !strings.EqualFold(u.Host, Host) {
			continue
		}
		action := strings.TrimSpace(u.Query().Get("action"))
		if action == "" {
			continue
		}
		return action, true
	}
	return "", false
}

func ForwardAction(baseURL, action string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/action?action=" + url.QueryEscape(action)
	resp, err := client.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return nil
}
