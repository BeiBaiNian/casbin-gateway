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

import React from "react";
import {Link} from "react-router-dom";
import {Button, Popconfirm, Space, Table, Tag, Tooltip} from "antd";
import {CheckCircleOutlined, CloseCircleOutlined, DeleteOutlined, EditOutlined, PlusOutlined} from "@ant-design/icons";
import i18next from "i18next";
import BaseListPage from "./BaseListPage";
import * as ChannelBackend from "./backend/ChannelBackend";
import * as Setting from "./Setting";

class ChannelListPage extends BaseListPage {
  UNSAFE_componentWillMount() {
    const pagination = {...this.state.pagination, current: 1, pageSize: 10};
    this.setState({pagination: pagination});
    this.fetch({pagination: pagination});
  }

  fetch = (params = {}) => {
    const sortField = params.sortField, sortOrder = params.sortOrder;
    if (!params.pagination) {
      params.pagination = {current: 1, pageSize: 10};
    }
    this.setState({loading: true});

    ChannelBackend.getChannels(this.props.account.name, params.pagination.current, params.pagination.pageSize, sortField, sortOrder).then(res => {
      this.setState({loading: false});
      if (res.status === "ok") {
        this.setState({
          data: res.data,
          pagination: {
            ...params.pagination,
            total: res.data2,
          },
        });
      } else {
        Setting.showMessage("error", `${i18next.t("channel:Failed to get channels")}: ${res.msg}`);
      }
    }).catch(error => {
      this.setState({loading: false});
      Setting.showMessage("error", `${i18next.t("channel:Failed to get channels")}: ${error}`);
    });
  };

  deleteChannel = (channel) => {
    ChannelBackend.deleteChannel(channel).then(res => {
      if (res.status === "error") {
        Setting.showMessage("error", `${i18next.t("channel:Failed to delete")}: ${res.msg}`);
        return;
      }

      Setting.showMessage("success", i18next.t("channel:Channel deleted successfully"));
      this.fetch({
        pagination: {
          ...this.state.pagination,
          current: this.state.pagination.current > 1 && this.state.data.length === 1 ? this.state.pagination.current - 1 : this.state.pagination.current,
        },
      });
    }).catch(error => {
      Setting.showMessage("error", `${i18next.t("channel:Failed to delete")}: ${error}`);
    });
  };

  newChannel() {
    const randomName = Setting.getRandomName();
    return {
      owner: this.props.account.name,
      name: `channel_${randomName}`,
      displayName: `New Channel - ${randomName}`,
      type: "openai",
      status: "enabled",
      models: [],
      priority: 0,
      baseUrl: "",
      apiKey: "",
    };
  }

  addChannel() {
    const newChannel = this.newChannel();
    ChannelBackend.addChannel(newChannel).then(res => {
      if (res.status === "error") {
        Setting.showMessage("error", `${i18next.t("channel:Failed to add")}: ${res.msg}`);
      } else {
        Setting.showMessage("success", i18next.t("channel:Channel added successfully"));
        this.fetch({pagination: this.state.pagination});
      }
    }).catch(error => {
      Setting.showMessage("error", `${i18next.t("channel:Failed to add")}: ${error}`);
    });
  }

  renderTypeTag(type) {
    const colorMap = {
      openai: "#10a37f",
      custom: "#8b5cf6",
    };
    return (
      <Tag color={colorMap[type] || "#595959"}>{colorMap[type] ? type : `${type} (${i18next.t("channel:Unsupported")})`}</Tag>
    );
  }

  renderStatusTag(status) {
    return status === "enabled" ? (
      <Tag icon={<CheckCircleOutlined />} color="success">{i18next.t("channel:Enabled")}</Tag>
    ) : (
      <Tag icon={<CloseCircleOutlined />} color="error">{i18next.t("channel:Disabled")}</Tag>
    );
  }

  renderTable(data) {
    const columns = [
      {
        title: i18next.t("general:Name"),
        dataIndex: "name",
        key: "name",
        width: "140px",
        ellipsis: true,
        // The server sorts and paginates, so sorting locally would only reorder
        // the current page.
        sorter: true,
        render: (text, record) => {
          return (
            <Tooltip title={text}>
              <Link to={`/channels/${record.owner}/${record.name}`}>
                {text}
              </Link>
            </Tooltip>
          );
        },
      },
      {
        title: i18next.t("general:Display name"),
        dataIndex: "displayName",
        key: "displayName",
        width: "180px",
        ellipsis: true,
        sorter: true,
        render: (text) => {
          if (!text) {return "-";}
          return (
            <Tooltip title={text}>
              <span>{text}</span>
            </Tooltip>
          );
        },
      },
      {
        title: i18next.t("channel:Type"),
        dataIndex: "type",
        key: "type",
        width: "90px",
        render: (text) => this.renderTypeTag(text),
      },
      {
        title: i18next.t("channel:Base URL"),
        dataIndex: "baseUrl",
        key: "baseUrl",
        width: "150px",
        ellipsis: true,
        render: (text) => {
          if (!text) {return "-";}
          return (
            <Tooltip title={text}>
              <span style={{fontFamily: "monospace", fontSize: "12px"}}>{text}</span>
            </Tooltip>
          );
        },
      },
      {
        title: i18next.t("channel:Models"),
        dataIndex: "models",
        key: "models",
        width: "180px",
        render: (models) => {
          if (!models || models.length === 0) {return "-";}
          return models.map(model => {
            return <Tag key={model} color="blue">{model}</Tag>;
          });
        },
      },
      {
        title: i18next.t("channel:Priority"),
        dataIndex: "priority",
        key: "priority",
        width: "80px",
        sorter: true,
      },
      {
        title: i18next.t("channel:Status"),
        dataIndex: "status",
        key: "status",
        width: "110px",
        render: (text) => this.renderStatusTag(text),
      },
      {
        title: i18next.t("general:Action"),
        key: "action",
        width: "180px",
        render: (text, record) => {
          return (
            <Space>
              <Button
                type="primary"
                size="small"
                icon={<EditOutlined />}
                onClick={() => this.props.history.push(`/channels/${record.owner}/${record.name}`)}
              >
                {i18next.t("general:Edit")}
              </Button>
              <Popconfirm
                title={`${i18next.t("general:Delete")} channel "${record.name}"?`}
                onConfirm={() => this.deleteChannel(record)}
                okText={i18next.t("general:OK")}
                cancelText={i18next.t("general:Cancel")}
              >
                <Button size="small" danger icon={<DeleteOutlined />}>{i18next.t("general:Delete")}</Button>
              </Popconfirm>
            </Space>
          );
        },
      },
    ];

    return (
      <div>
        <Table
          className="channel-table"
          columns={columns}
          dataSource={data}
          // An admin sees channels across owners, where names may collide.
          rowKey={(record) => `${record.owner}/${record.name}`}
          bordered
          pagination={this.state.pagination}
          title={() => (
            <div>
              {i18next.t("channel:Channels")}&nbsp;&nbsp;&nbsp;&nbsp;
              <Button type="primary" size="small" icon={<PlusOutlined />} onClick={this.addChannel.bind(this)}>
                {i18next.t("channel:New Channel")}
              </Button>
            </div>
          )}
          loading={this.state.loading}
          onChange={this.handleTableChange}
        />
      </div>
    );
  }
}

export default ChannelListPage;
