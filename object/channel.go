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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// ErrNoChannelAvailable is returned by GetChannelsByModel when no enabled
// channel matches the requested model name. It is a sentinel error so
// callers can distinguish "no match" (client error, HTTP 400) from
// database failures (server error, HTTP 502).
var ErrNoChannelAvailable = errors.New("no available channel")

// ApiKeyMask is what the API returns in place of a stored API key. Sending it
// back in an update means "keep the existing key"; sending anything else
// (including an empty string) overwrites the stored key.
const ApiKeyMask = "***"

// AnthropicVersion is the API version sent upstream when the client did not
// pick one. The Anthropic API rejects a request without it.
const AnthropicVersion = "2023-06-01"

// apiKeyEncryptionSecret is empty when encryption is off, which keeps keys
// stored as plaintext like before.
func apiKeyEncryptionSecret() string {
	return conf.GetConfigString("apiKeyEncryptionKey")
}

// apiKeyAad binds the ciphertext to its own row, so a value copied into another
// channel's api_key column no longer decrypts.
func apiKeyAad(channel *Channel) string {
	return channel.GetId()
}

// encryptApiKey needs channel.Owner and channel.Name to be set already.
func encryptApiKey(channel *Channel) error {
	encrypted, err := util.EncryptWithKey(apiKeyEncryptionSecret(), channel.ApiKey, apiKeyAad(channel))
	if err != nil {
		return err
	}
	channel.ApiKey = encrypted
	return nil
}

// decryptChannel restores the plaintext ApiKey on a channel just read from the
// database. A failure leaves the stored value in place rather than dropping the
// channel, but is logged: otherwise a changed key looks exactly like a healthy
// channel whose upstream answers 401.
func decryptChannel(channel *Channel) {
	if channel == nil {
		return
	}

	secret := apiKeyEncryptionSecret()
	stored := channel.ApiKey

	plain, err := util.DecryptWithKey(secret, stored, apiKeyAad(channel))
	if err != nil {
		fmt.Printf("decryptChannel(): channel [%s]: %v\n", channel.GetId(), err)
		return
	}
	channel.ApiKey = plain

	if util.NeedsReEncryption(secret, stored) {
		upgradeStoredApiKey(channel)
	}
}

// apiKeyUpgrades collapses concurrent upgrades of the same row: GetChannelsByModel()
// runs on every proxied request.
var apiKeyUpgrades sync.Map

// upgradeStoredApiKey rewrites a plaintext or older-format key in the current
// format. Only api_key is touched, so UpdatedTime keeps reflecting the last real
// edit. A failure is logged and ignored, and retried on the next read.
func upgradeStoredApiKey(channel *Channel) {
	id := channel.GetId()
	if _, busy := apiKeyUpgrades.LoadOrStore(id, struct{}{}); busy {
		return
	}
	defer apiKeyUpgrades.Delete(id)

	encrypted, err := util.EncryptWithKey(apiKeyEncryptionSecret(), channel.ApiKey, apiKeyAad(channel))
	if err != nil {
		fmt.Printf("upgradeStoredApiKey(): channel [%s]: %v\n", id, err)
		return
	}

	_, err = ormer.Engine.ID(core.PK{channel.Owner, channel.Name}).
		Cols("api_key").Update(&Channel{ApiKey: encrypted})
	if err != nil {
		fmt.Printf("upgradeStoredApiKey(): channel [%s]: %v\n", id, err)
	}
}

func decryptChannels(channels []*Channel) {
	for _, channel := range channels {
		decryptChannel(channel)
	}
}

const (
	maxChannelModels     = 200
	maxChannelModelChars = 100
)

var (
	channelTypes    = []string{"openai", "custom", "anthropic"}
	channelStatuses = []string{"enabled", "disabled"}
)

// The two ways a channel authenticates upstream. In ChannelAuthClient mode the
// gateway forwards the credentials the caller sent instead of a stored key, so
// an agent already signed in with a subscription keeps its own login.
const (
	ChannelAuthChannel = "channel"
	ChannelAuthClient  = "client"
)

var channelAuthModes = []string{ChannelAuthChannel, ChannelAuthClient}

// UsesClientAuth reports whether the channel authenticates with the caller's
// own credentials. An empty AuthMode is a row written before the field existed.
func UsesClientAuth(channel *Channel) bool {
	return channel.AuthMode == ChannelAuthClient
}

// The wire formats the gateway can speak. A request in one of them can only be
// forwarded to a channel whose upstream speaks the same one.
const (
	ProtocolOpenAi    = "openai"
	ProtocolAnthropic = "anthropic"
)

// IsChannelTypeSupported reports whether the gateway can talk to the channel's
// upstream.
func IsChannelTypeSupported(channel *Channel) bool {
	return containsString(channelTypes, channel.Type)
}

// ChannelProtocol is the wire format a channel's upstream speaks. Everything
// that is not Anthropic is reached with an OpenAI-formatted request.
func ChannelProtocol(channel *Channel) string {
	if channel.Type == "anthropic" {
		return ProtocolAnthropic
	}
	return ProtocolOpenAi
}

func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// Channel is an upstream AI provider channel. (Milestone 1.1)
type Channel struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	DisplayName string `xorm:"varchar(100)" json:"displayName"`
	Type        string `xorm:"varchar(100)" json:"type"`
	BaseUrl     string `xorm:"varchar(255)" json:"baseUrl"`
	// ApiKey holds base64 ciphertext, not the bare key, when
	// "apiKeyEncryptionKey" is set in app.conf, hence the wider column.
	ApiKey string `xorm:"varchar(1000)" json:"apiKey"`
	// AuthMode selects whose credentials reach the upstream, see UsesClientAuth.
	AuthMode string `xorm:"varchar(100)" json:"authMode"`
	// Models is JSON-serialized by xorm, so it needs a text column rather than
	// a varchar: the serialized form is longer than the joined model names.
	Models []string `xorm:"mediumtext" json:"models"`
	// TODO(1.2): Priority routing strategy will be defined in milestone 1.2.
	Priority int    `xorm:"int" json:"priority"`
	Status   string `xorm:"varchar(100)" json:"status"`
}

func (channel *Channel) GetId() string {
	return fmt.Sprintf("%s/%s", channel.Owner, channel.Name)
}

func GetChannels(owner string) ([]*Channel, error) {
	channels := []*Channel{}
	session := GetSession(owner, -1, -1, "", "", "", "")
	err := session.Find(&channels)
	decryptChannels(channels)
	return channels, err
}

func GetChannelCount(owner, field, value string) (int64, error) {
	session := GetSession(owner, -1, -1, field, value, "", "")
	return session.Count(&Channel{})
}

func GetPaginationChannels(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Channel, error) {
	channels := []*Channel{}
	session := GetSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Find(&channels)
	decryptChannels(channels)
	return channels, err
}

func getChannel(owner, name string) (*Channel, error) {
	channel := &Channel{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(channel)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	decryptChannel(channel)
	return channel, nil
}

func GetChannel(id string) (*Channel, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	return getChannel(owner, name)
}

// GetMaskedChannel returns a copy of the channel with the API key replaced by
// ApiKeyMask, so the stored key never reaches the browser.
func GetMaskedChannel(channel *Channel) *Channel {
	if channel == nil {
		return nil
	}

	masked := *channel
	if masked.ApiKey != "" {
		masked.ApiKey = ApiKeyMask
	}
	return &masked
}

func GetMaskedChannels(channels []*Channel) []*Channel {
	maskedChannels := make([]*Channel, 0, len(channels))
	for _, channel := range channels {
		maskedChannels = append(maskedChannels, GetMaskedChannel(channel))
	}
	return maskedChannels
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
	if channel.AuthMode == "" {
		channel.AuthMode = ChannelAuthChannel
	}

	if !containsString(channelTypes, channel.Type) {
		return fmt.Errorf("invalid channel type: %s", channel.Type)
	}
	if !containsString(channelStatuses, channel.Status) {
		return fmt.Errorf("invalid channel status: %s", channel.Status)
	}
	if !containsString(channelAuthModes, channel.AuthMode) {
		return fmt.Errorf("invalid channel auth mode: %s", channel.AuthMode)
	}

	// A channel that forwards the caller's credentials has no use for a stored
	// key, and one left behind would be sent upstream again after a switch back.
	if channel.AuthMode == ChannelAuthClient {
		channel.ApiKey = ""
	}

	if channel.BaseUrl != "" {
		if err := validateBaseUrl(channel.BaseUrl); err != nil {
			return err
		}
	}

	if len(channel.Models) > maxChannelModels {
		return fmt.Errorf("too many models: %d, at most %d are allowed", len(channel.Models), maxChannelModels)
	}
	for _, model := range channel.Models {
		if strings.TrimSpace(model) == "" {
			return fmt.Errorf("model name cannot be empty")
		}
		if len(model) > maxChannelModelChars {
			return fmt.Errorf("model name is too long: %s", model)
		}
	}

	return nil
}

func validateBaseUrl(baseUrl string) error {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return fmt.Errorf("invalid base URL: %s", err.Error())
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid base URL: only the http and https schemes are supported")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("invalid base URL: the host is empty")
	}
	return nil
}

// BuildOpenAiUrl joins an OpenAI-compatible endpoint onto a channel base URL.
// The base URL may be bare, already carry the /v1 prefix or already end with
// the endpoint itself; none of those forms are doubled.
func BuildOpenAiUrl(baseUrl string, endpoint string) (string, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %s", err.Error())
	}

	path := strings.TrimSuffix(strings.TrimRight(u.Path, "/"), endpoint)
	if !strings.HasSuffix(path, "/v1") {
		path += "/v1"
	}

	u.Path = path + endpoint
	u.RawPath = ""
	return u.String(), nil
}

// BuildAnthropicUrl joins an Anthropic endpoint onto a channel base URL. Unlike
// an OpenAI base URL, an Anthropic one is bare and the endpoint carries the /v1
// prefix; a base URL that already has one is not doubled.
func BuildAnthropicUrl(baseUrl string, endpoint string) (string, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %s", err.Error())
	}

	path := strings.TrimSuffix(strings.TrimRight(u.Path, "/"), endpoint)
	path = strings.TrimSuffix(strings.TrimRight(path, "/"), "/v1")

	u.Path = path + endpoint
	u.RawPath = ""
	return u.String(), nil
}

// BuildChannelUrl is the upstream URL a request in the given protocol is sent
// to. The endpoint is the protocol's own, not a shared one.
func BuildChannelUrl(baseUrl string, protocol string, endpoint string) (string, error) {
	if protocol == ProtocolAnthropic {
		return BuildAnthropicUrl(baseUrl, endpoint)
	}
	return BuildOpenAiUrl(baseUrl, endpoint)
}

// AppendQuery puts the query of the client request back on an upstream URL.
// The query selects a variant of the endpoint — the Anthropic clients ask for
// the beta one with "?beta=true" — so dropping it would forward a different
// request than the one that was made. A base URL carrying a query of its own
// keeps it, with the client's appended.
func AppendQuery(rawUrl string, rawQuery string) string {
	if rawQuery == "" {
		return rawUrl
	}

	u, err := url.Parse(rawUrl)
	if err != nil {
		return rawUrl
	}
	if u.RawQuery != "" {
		rawQuery = u.RawQuery + "&" + rawQuery
	}
	u.RawQuery = rawQuery
	return u.String()
}

// SetChannelAuth puts the channel's credentials on an upstream request, in the
// header the channel's protocol authenticates with.
func SetChannelAuth(header http.Header, channel *Channel) {
	// The caller's own credentials are already on the request, and the proxy
	// copies them across itself.
	if UsesClientAuth(channel) {
		return
	}

	if ChannelProtocol(channel) == ProtocolAnthropic {
		header.Set("X-Api-Key", channel.ApiKey)
		return
	}
	header.Set("Authorization", "Bearer "+channel.ApiKey)
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

	if err := encryptApiKey(channel); err != nil {
		return false, err
	}

	affected, err := ormer.Engine.Insert(channel)
	return affected != 0, err
}

func UpdateChannel(id string, channel *Channel) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	if stored, err := getChannel(owner, name); err != nil {
		return false, err
	} else if stored == nil {
		return false, nil
	}

	if err := validateChannel(channel); err != nil {
		return false, err
	}

	channel.Owner = owner
	channel.Name = name
	channel.UpdatedTime = util.GetCurrentTime()

	session := ormer.Engine.ID(core.PK{owner, name})
	// The browser only ever sees the mask, so getting it back means the user
	// did not touch the field. Any other value (including "") is written, which
	// is what makes clearing a key possible.
	if channel.ApiKey == ApiKeyMask {
		session = session.Omit("api_key")
	} else if err := encryptApiKey(channel); err != nil {
		return false, err
	}

	affected, err := session.AllCols().Update(channel)
	if err == nil {
		// The edit may be the fix for whatever the proxy last saw, so the
		// channel starts from a clean slate.
		ClearChannelHealth(channel.GetId())
	}
	return affected != 0, err
}

func DeleteChannel(channel *Channel) (bool, error) {
	affected, err := ormer.Engine.ID(core.PK{channel.Owner, channel.Name}).Delete(&Channel{})
	if err == nil {
		ClearChannelHealth(channel.GetId())
	}
	return affected != 0, err
}

// maxProbeBody caps what is read back from an upstream: a model list from an
// aggregator is long, and an error page can be arbitrarily long.
const maxProbeBody = 1 << 20

// channelProbe is what a read-only GET against a channel's models endpoint
// returned. The same probe answers both "is this upstream reachable" and
// "which models does it serve".
type channelProbe struct {
	statusCode int
	status     string
	body       []byte
}

func (probe *channelProbe) ok() bool {
	return probe.statusCode >= 200 && probe.statusCode < 300
}

// probeChannel performs the read-only GET against the channel's models
// endpoint. The channel is used as given rather than read back from the
// database, so a channel that is not saved yet can be probed too.
func probeChannel(channel *Channel) (*channelProbe, error) {
	if !IsChannelTypeSupported(channel) {
		return nil, fmt.Errorf("the %s channel type is not supported", channel.Type)
	}

	if channel.BaseUrl == "" {
		return nil, errors.New("the base URL is empty")
	}
	if err := validateBaseUrl(channel.BaseUrl); err != nil {
		return nil, err
	}

	protocol := ChannelProtocol(channel)
	probeEndpoint := "/models"
	if protocol == ProtocolAnthropic {
		probeEndpoint = "/v1/models"
	}

	probeUrl, err := BuildChannelUrl(channel.BaseUrl, protocol, probeEndpoint)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, probeUrl, nil)
	if err != nil {
		return nil, err
	}
	if protocol == ProtocolAnthropic {
		req.Header.Set("Anthropic-Version", AnthropicVersion)
	}
	if channel.ApiKey != "" {
		SetChannelAuth(req.Header, channel)
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: proxy.Transport(),
		// Do not follow redirects, so the reported status is the one the
		// configured base URL actually returns.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	resp, err := client.Do(req)
	if err != nil {
		// Unwrap the *url.Error so the reason is not buried behind the URL.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, urlErr.Err
		}
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProbeBody))
	if err != nil {
		return nil, err
	}

	return &channelProbe{statusCode: resp.StatusCode, status: resp.Status, body: body}, nil
}

// TestChannelConnectivity performs a read-only probe against the channel's
// upstream. It returns whether the probe succeeded, the upstream HTTP status
// code (0 when no response was received) and a human-readable message.
func TestChannelConnectivity(channel *Channel) (bool, int, string) {
	stored, err := getChannel(channel.Owner, channel.Name)
	if err != nil {
		return false, 0, err.Error()
	}
	if stored == nil {
		return false, 0, "the channel does not exist"
	}

	probe, err := probeChannel(stored)
	if err != nil {
		return false, 0, err.Error()
	}

	// A client-auth channel has no key to probe with, so an upstream that
	// rejects the unauthenticated probe has still proven it is reachable.
	if UsesClientAuth(stored) && (probe.statusCode == http.StatusUnauthorized || probe.statusCode == http.StatusForbidden) {
		return true, probe.statusCode, "reachable, and authenticated with the caller's own credentials"
	}

	return probe.ok(), probe.statusCode, probe.status
}

// FetchChannelModels lists what the channel's upstream reports at its models
// endpoint, so the model names do not have to be typed by hand.
func FetchChannelModels(channel *Channel) ([]string, error) {
	probe, err := probeChannel(channel)
	if err != nil {
		return nil, err
	}
	if !probe.ok() {
		return nil, fmt.Errorf("the upstream answered %s%s", probe.status, probeDetail(probe.body))
	}

	models, err := parseModelList(probe.body)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, errors.New("the upstream did not report any model")
	}
	return models, nil
}

// parseModelList reads the model ids out of a models response. OpenAI and
// Anthropic both answer {"data": [{"id": ...}]}, and an OpenAI-compatible
// vendor follows OpenAI.
func parseModelList(body []byte) ([]string, error) {
	var payload struct {
		Data []struct {
			Id string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("the upstream did not answer with a model list: %s", err.Error())
	}

	models := []string{}
	seen := map[string]bool{}
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.Id)
		if id == "" || len(id) > maxChannelModelChars || seen[id] {
			continue
		}
		seen[id] = true
		models = append(models, id)
	}
	// The upstream order is kept: Anthropic returns its newest model first.
	return models, nil
}

// probeDetail is the upstream's own error text, trimmed to what fits in a
// toast: a status code alone rarely says which of key, URL or plan is wrong.
func probeDetail(body []byte) string {
	text := strings.Join(strings.Fields(string(body)), " ")
	if text == "" {
		return ""
	}
	if runes := []rune(text); len(runes) > 200 {
		text = string(runes[:200]) + "..."
	}
	return ": " + text
}

// GetChannelsByModel returns every enabled channel that supports the given
// model name, ordered by priority (ascending, so the lowest value comes first)
// so that the caller can fail over from one channel to the next. It queries all
// channels globally (no owner filter) because /v1/chat/completions is an
// unauthenticated public endpoint.
func GetChannelsByModel(model string) ([]*Channel, error) {
	channels := []*Channel{}
	err := ormer.Engine.Where("status = ?", "enabled").Asc("priority").Find(&channels)
	if err != nil {
		return nil, fmt.Errorf("channel query failed: %w", err)
	}

	// The models are JSON-serialized into a single column, so the match cannot
	// be pushed down into the query.
	matchedChannels := []*Channel{}
	// A channel authenticated with the caller's own credentials cannot know
	// which models the account behind them may use, so an empty model list
	// there means "any model". Those channels are tried after the ones that
	// name the model, so a wildcard never takes traffic from an exact match.
	wildcardChannels := []*Channel{}
	for _, channel := range channels {
		if len(channel.Models) == 0 {
			if UsesClientAuth(channel) {
				wildcardChannels = append(wildcardChannels, channel)
			}
			continue
		}
		for _, channelModel := range channel.Models {
			if channelModel == model {
				matchedChannels = append(matchedChannels, channel)
				break
			}
		}
	}
	matchedChannels = append(matchedChannels, wildcardChannels...)

	decryptChannels(matchedChannels)

	if len(matchedChannels) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoChannelAvailable, model)
	}
	return matchedChannels, nil
}
