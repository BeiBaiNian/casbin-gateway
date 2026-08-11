// Copyright 2024 The casbin Authors. All Rights Reserved.
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

type LlmLog struct {
	Id               int     `xorm:"int notnull pk autoincr" json:"id"`
	Owner            string  `xorm:"varchar(100) notnull" json:"owner"`
	CreatedTime      string  `xorm:"varchar(100) notnull" json:"createdTime"`
	TokenName        string  `xorm:"varchar(100)" json:"tokenName"`
	Channel          string  `xorm:"varchar(100)" json:"channel"`
	Model            string  `xorm:"varchar(100)" json:"model"`
	PromptTokens     int     `xorm:"int" json:"promptTokens"`
	CompletionTokens int     `xorm:"int" json:"completionTokens"`
	Cost             float64 `xorm:"float" json:"cost"`
	Status           string  `xorm:"varchar(100)" json:"status"`
	ErrorMessage     string  `xorm:"varchar(500)" json:"errorMessage"`
}

func AddLlmLog(log *LlmLog) (bool, error) {
	affected, err := ormer.Engine.Insert(log)
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func GetLlmLogs(owner string) ([]*LlmLog, error) {
	llmLogs := []*LlmLog{}
	err := ormer.Engine.Asc("id").Find(&llmLogs, &LlmLog{Owner: owner})
	if err != nil {
		return nil, err
	}

	return llmLogs, nil
}

func GetLlmLogCount(owner, field, value string) (int64, error) {
	session := GetSession(owner, -1, -1, field, value, "", "")
	return session.Count(&LlmLog{})
}

func GetPaginationLlmLogs(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*LlmLog, error) {
	llmLogs := []*LlmLog{}
	session := GetSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Where("owner = ? or owner = ?", "admin", owner).Find(&llmLogs)
	if err != nil {
		return llmLogs, err
	}

	return llmLogs, nil
}
