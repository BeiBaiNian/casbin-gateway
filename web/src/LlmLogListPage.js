// Copyright 2021 The casbin Authors. All Rights Reserved.
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

import React from "react";
import {Table, Tag} from "antd";
import * as Setting from "./Setting";
import * as LlmLogBackend from "./backend/LlmLogBackend";
import i18next from "i18next";
import BaseListPage from "./BaseListPage";

class LlmLogListPage extends BaseListPage {

  UNSAFE_componentWillMount() {
    this.setState({
      pagination: {
        ...this.state.pagination,
        current: 1,
        pageSize: 10,
      },
    });
    this.fetch({pagination: this.state.pagination});
  }

  fetch = (params = {}) => {
    const sortField = params.sortField, sortOrder = params.sortOrder;
    if (!params.pagination) {
      params.pagination = {current: 1, pageSize: 10};
    }
    this.setState({loading: true});
    LlmLogBackend.getLlmLogs(this.props.account.name, params.pagination.current, params.pagination.pageSize, sortField, sortOrder)
      .then((res) => {
        this.setState({
          loading: false,
        });
        if (res.status === "ok") {
          this.setState({
            data: res.data,
            pagination: {
              ...params.pagination,
              total: res.data2,
            },
          });
        } else {
          Setting.showMessage("error", `Failed to get LLM logs: ${res.msg}`);
        }
      });
  };

  renderTable(data) {
    const columns = [
      {
        title: i18next.t("general:ID"),
        dataIndex: "id",
        key: "id",
        width: "30px",
        sorter: (a, b) => a.id - b.id,
      },
      {
        title: i18next.t("general:Owner"),
        dataIndex: "owner",
        key: "owner",
        width: "30px",
        sorter: (a, b) => a.owner.localeCompare(b.owner),
      },
      {
        title: i18next.t("general:Created time"),
        dataIndex: "createdTime",
        key: "createdTime",
        width: "70px",
        sorter: (a, b) => a.createdTime.localeCompare(b.createdTime),
        render: (text, record, index) => {
          return Setting.getFormattedDate(text);
        },
      },
      {
        title: i18next.t("general:Token Name"),
        dataIndex: "tokenName",
        key: "tokenName",
        width: "60px",
        sorter: (a, b) => a.tokenName.localeCompare(b.tokenName),
      },
      {
        title: i18next.t("general:Channel"),
        dataIndex: "channel",
        key: "channel",
        width: "50px",
        sorter: (a, b) => a.channel.localeCompare(b.channel),
      },
      {
        title: i18next.t("general:Model"),
        dataIndex: "model",
        key: "model",
        width: "50px",
        sorter: (a, b) => a.model.localeCompare(b.model),
      },
      {
        title: i18next.t("general:Prompt Tokens"),
        dataIndex: "promptTokens",
        key: "promptTokens",
        width: "40px",
        sorter: (a, b) => a.promptTokens - b.promptTokens,
      },
      {
        title: i18next.t("general:Completion Tokens"),
        dataIndex: "completionTokens",
        key: "completionTokens",
        width: "40px",
        sorter: (a, b) => a.completionTokens - b.completionTokens,
      },
      {
        title: i18next.t("general:Cost"),
        dataIndex: "cost",
        key: "cost",
        width: "30px",
        sorter: (a, b) => a.cost - b.cost,
      },
      {
        title: i18next.t("general:Status"),
        dataIndex: "status",
        key: "status",
        width: "40px",
        sorter: (a, b) => a.status.localeCompare(b.status),
        render: (text, record, index) => {
          let color = "green";
          if (text === "fail") {
            color = "red";
          }
          return <Tag color={color}>{text}</Tag>;
        },
      },
      {
        title: i18next.t("general:Error Message"),
        dataIndex: "errorMessage",
        key: "errorMessage",
        width: "120px",
        sorter: (a, b) => a.errorMessage.localeCompare(b.errorMessage),
      },
    ];

    return (
      <div>
        <Table columns={columns} dataSource={data} rowKey="id" size="middle" bordered pagination={this.state.pagination}
          title={() => (
            <div>
              {i18next.t("general:LLM Logs")}&nbsp;&nbsp;&nbsp;&nbsp;
            </div>
          )}
          loading={this.state.loading}
          onChange={this.handleTableChange}
        />
      </div>
    );
  }

}

export default LlmLogListPage;
