// Copyright 2023 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// Channel is an upstream AI provider channel. (Milestone 1.1)
type Channel struct {
	Owner       string   `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string   `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string   `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string   `xorm:"varchar(100)" json:"updatedTime"`
	DisplayName string   `xorm:"varchar(100)" json:"displayName"`
	Type        string   `xorm:"varchar(100)" json:"type"`
	BaseUrl     string   `xorm:"varchar(255)" json:"baseUrl"`
	// TODO(1.3): ApiKey is stored as plaintext; AES encryption will be added in milestone 1.3.
	ApiKey  string   `xorm:"varchar(500)" json:"apiKey"`
	Models  []string `xorm:"varchar(500)" json:"models"`
	// TODO(1.2): Priority routing strategy will be defined in milestone 1.2.
	Priority int    `xorm:"int" json:"priority"`
	Status   string `xorm:"varchar(100)" json:"status"`
}

func GetChannels(owner string) ([]*Channel, error) {
	channels := []*Channel{}
	engine := ormer.Engine.Desc("created_time")
	if owner != "" {
		engine = engine.Where("owner = ?", owner)
	}
	err := engine.Find(&channels)
	return channels, err
}

func GetChannelCount(owner string) (int64, error) {
	if owner == "" {
		return ormer.Engine.Count(&Channel{})
	}
	return ormer.Engine.Where("owner = ?", owner).Count(&Channel{})
}

func GetPaginationChannels(owner string, offset, limit int) ([]*Channel, error) {
	channels := []*Channel{}
	engine := ormer.Engine.Desc("created_time").Limit(limit, offset)
	if owner != "" {
		engine = engine.Where("owner = ?", owner)
	}
	err := engine.Find(&channels)
	return channels, err
}

func getChannel(owner, name string) (*Channel, error) {
	channel := &Channel{Owner: owner, Name: name}
	ok, err := ormer.Engine.Get(channel)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return channel, nil
}

func GetChannel(id string) (*Channel, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	return getChannel(owner, name)
}

func GetMaskedChannel(channel *Channel) *Channel {
	if channel != nil && channel.ApiKey != "" {
		channel.ApiKey = "***"
	}
	return channel
}

func GetMaskedChannels(channels []*Channel) []*Channel {
	for _, channel := range channels {
		GetMaskedChannel(channel)
	}
	return channels
}

func validateChannel(channel *Channel) error {
	if channel.Type == "" {
		channel.Type = "openai"
	}
	if channel.Status == "" {
		channel.Status = "enabled"
	}
	if channel.Models == nil {
		channel.Models = []string{}
	}

	if channel.Type != "openai" && channel.Type != "claude" && channel.Type != "gemini" && channel.Type != "custom" {
		return fmt.Errorf("invalid channel type")
	}
	if channel.Status != "enabled" && channel.Status != "disabled" {
		return fmt.Errorf("invalid channel status")
	}
	if len(strings.Join(channel.Models, "")) > 500 {
		return fmt.Errorf("models exceed 500 characters")
	}

	return nil
}

func AddChannel(channel *Channel) (bool, error) {
	if err := validateChannel(channel); err != nil {
		return false, err
	}

	now := util.GetCurrentTime()
	if channel.CreatedTime == "" {
		channel.CreatedTime = now
	}
	channel.UpdatedTime = now

	if channel.ApiKey != "" {
		encrypted, err := EncryptApiKey(channel.ApiKey)
		if err != nil {
			return false, fmt.Errorf("failed to encrypt api key: %v", err)
		}
		channel.ApiKey = encrypted
	}

	n, err := ormer.Engine.Insert(channel)
	return n != 0, err
}

func UpdateChannel(id string, channel *Channel) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)

	if err := validateChannel(channel); err != nil {
		return false, err
	}

	channel.Owner = owner
	channel.Name = name
	channel.UpdatedTime = util.GetCurrentTime()

	var n int64
	var err error
	if channel.ApiKey == "***" || channel.ApiKey == "" {
		n, err = ormer.Engine.ID(core.PK{owner, name}).Omit("api_key").AllCols().Update(channel)
	} else {
		encrypted, encErr := EncryptApiKey(channel.ApiKey)
		if encErr != nil {
			return false, fmt.Errorf("failed to encrypt api key: %v", encErr)
		}
		channel.ApiKey = encrypted
		n, err = ormer.Engine.ID(core.PK{owner, name}).AllCols().Update(channel)
	}

	return n != 0, err
}

func DeleteChannel(channel *Channel) (bool, error) {
	n, err := ormer.Engine.ID(core.PK{channel.Owner, channel.Name}).Delete(&Channel{})
	return n != 0, err
}

func IsAllowedPort(port int) bool {
	return port == 80 || port == 443 || (port >= 8000 && port <= 9999)
}

func TestChannelConnectivity(channel *Channel, apiKeyOverride string) (bool, int, string) {
	stored, err := getChannel(channel.Owner, channel.Name)
	if err != nil || stored == nil {
		return false, 0, "channel not found"
	}

	if channel.Type == "claude" {
		return false, 0, "claude type not yet supported in this stage"
	}
	if channel.Type == "gemini" {
		return false, 0, "gemini type not yet supported in this stage"
	}

	key := apiKeyOverride
	if key == "" || key == "***" {
		decrypted, decErr := DecryptApiKey(stored.ApiKey)
		if decErr == nil {
			key = decrypted
		} else {
			key = stored.ApiKey
		}
	}

	u, err := url.Parse(stored.BaseUrl)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return false, 0, "connection blocked by security policy"
	}

	path := strings.TrimRight(stored.BaseUrl, "/")
	if stored.Type == "openai" {
		path += "/v1/models"
	}

	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return false, 0, "connection blocked by security policy"
	}
	if stored.Type == "openai" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	host := u.Hostname()
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, e := net.SplitHostPort(addr)
			if e != nil {
				return nil, fmt.Errorf("connection blocked by security policy")
			}

			p, e := strconv.Atoi(port)
			if e != nil || !IsAllowedPort(p) {
				return nil, fmt.Errorf("connection blocked by security policy")
			}

			ips, e := net.LookupIP(host)
			if e != nil || len(ips) == 0 {
				return nil, fmt.Errorf("connection blocked by security policy")
			}

			for _, ip := range ips {
				if util.IsIntranetIp(ip.String()) {
					return nil, fmt.Errorf("connection blocked by security policy")
				}
			}

			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
	}

	if u.Scheme == "https" {
		transport.TLSClientConfig = &tls.Config{ServerName: host}
	}

	client := &http.Client{
		Transport:     transport,
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, 0, "connection blocked by security policy"
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.StatusCode >= 200 && resp.StatusCode < 300, resp.StatusCode, resp.Status
}

// GetChannelByModel returns the highest-priority enabled channel that supports
// the given model name. It queries all channels globally (no owner filter)
// because /v1/chat/completions is an unauthenticated public endpoint.
// Corresponds to PM section 3.4.
func GetChannelByModel(model string) (*Channel, error) {
	channels := []*Channel{}
	err := ormer.Engine.Where("status = ?", "enabled").Asc("priority").Find(&channels)
	if err != nil {
		return nil, err
	}
	for _, ch := range channels {
		for _, m := range ch.Models {
			if m == model {
				// Decrypt the API key for forwarding.
				decrypted, decErr := DecryptApiKey(ch.ApiKey)
				if decErr == nil {
					ch.ApiKey = decrypted
				}
				return ch, nil
			}
		}
	}
	return nil, fmt.Errorf("no available channel for model: %s", model)
}

// MigrateChannelApiKeys encrypts all existing plaintext API keys in the channels table.
// It is idempotent — encrypted keys are left unchanged.
func MigrateChannelApiKeys() {
	channels := []*Channel{}
	err := ormer.Engine.Find(&channels)
	if err != nil {
		log.Printf("MigrateChannelApiKeys: failed to query channels: %v", err)
		return
	}

	for _, ch := range channels {
		if ch.ApiKey == "" {
			continue
		}

		// Try to decrypt — if it fails, the key is plaintext and needs encryption.
		_, decErr := DecryptApiKey(ch.ApiKey)
		if decErr == nil {
			// Already encrypted, skip.
			continue
		}

		encrypted, encErr := EncryptApiKey(ch.ApiKey)
		if encErr != nil {
			log.Printf("MigrateChannelApiKeys: failed to encrypt channel %s/%s: %v", ch.Owner, ch.Name, encErr)
			continue
		}

		ch.ApiKey = encrypted
		_, updateErr := ormer.Engine.ID(core.PK{ch.Owner, ch.Name}).Cols("api_key").Update(ch)
		if updateErr != nil {
			log.Printf("MigrateChannelApiKeys: failed to update channel %s/%s: %v", ch.Owner, ch.Name, updateErr)
			continue
		}

		prefix := ch.ApiKey
		if len(prefix) > 8 {
			prefix = prefix[:4] + "..." + prefix[len(prefix)-4:]
		}
		log.Printf("migrated channel %s/%s key prefix: %s", ch.Owner, ch.Name, prefix)
	}
}
