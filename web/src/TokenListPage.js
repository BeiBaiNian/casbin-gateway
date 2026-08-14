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
import {Alert, Button, Input, Modal, Popconfirm, Select, Space, Table, Tag, Tooltip} from "antd";
import {CheckCircleOutlined, CloseCircleOutlined, CopyOutlined, DeleteOutlined, EditOutlined, PlusOutlined} from "@ant-design/icons";
import i18next from "i18next";
import BaseListPage from "./BaseListPage";
import * as ChannelBackend from "./backend/ChannelBackend";
import * as TokenBackend from "./backend/TokenBackend";
import * as Setting from "./Setting";

const {Option} = Select;

class TokenListPage extends BaseListPage {
  constructor(props) {
    super(props);
    this.state = {
      ...this.state,
      channelModels: [],
      createModalVisible: false,
      creating: false,
      newTokenName: "",
      newTokenModels: [],
      createdSecretKey: "",
    };
  }

  UNSAFE_componentWillMount() {
    const pagination = {...this.state.pagination, current: 1, pageSize: 10};
    this.setState({pagination: pagination});
    this.fetch({pagination: pagination});
    this.fetchChannelModels();
  }

  fetchChannelModels() {
    // Collect every model offered by the current owner's channels as
    // selectable options for token creation and editing.
    ChannelBackend.getChannels(this.props.account.name, 1, 1000).then(res => {
      if (res.status === "ok") {
        const modelSet = new Set();
        res.data.forEach(channel => {
          (channel.models || []).forEach(model => modelSet.add(model));
        });
        this.setState({channelModels: Array.from(modelSet).sort()});
      }
    });
  }

  fetch = (params = {}) => {
    if (!params.pagination) {
      params.pagination = {current: 1, pageSize: 10};
    }
    this.setState({loading: true});

    TokenBackend.getTokens(this.props.account.name, params.pagination.current, params.pagination.pageSize).then(res => {
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
        Setting.showMessage("error", `${i18next.t("token:Failed to get tokens")}: ${res.msg}`);
      }
    }).catch(error => {
      this.setState({loading: false});
      Setting.showMessage("error", `${i18next.t("token:Failed to get tokens")}: ${error}`);
    });
  };

  deleteToken = (token) => {
    TokenBackend.deleteToken(token).then(res => {
      if (res.status === "error") {
        Setting.showMessage("error", `${i18next.t("token:Failed to delete")}: ${res.msg}`);
        return;
      }

      Setting.showMessage("success", i18next.t("token:Token deleted successfully"));
      this.fetch({
        pagination: {
          ...this.state.pagination,
          current: this.state.pagination.current > 1 && this.state.data.length === 1 ? this.state.pagination.current - 1 : this.state.pagination.current,
        },
      });
    }).catch(error => {
      Setting.showMessage("error", `${i18next.t("token:Failed to delete")}: ${error}`);
    });
  };

  openCreateModal() {
    this.setState({
      createModalVisible: true,
      newTokenName: "",
      newTokenModels: [],
      createdSecretKey: "",
    });
  }

  closeCreateModal() {
    this.setState({createModalVisible: false, createdSecretKey: ""});
  }

  addToken() {
    if (!this.state.newTokenName || this.state.newTokenName.trim() === "") {
      Setting.showMessage("error", i18next.t("token:Name is required"));
      return;
    }

    this.setState({creating: true});
    TokenBackend.addToken({
      owner: this.props.account.name,
      name: this.state.newTokenName.trim(),
      displayName: this.state.newTokenName.trim(),
      status: "enabled",
      allowedModels: this.state.newTokenModels,
      rateLimit: 0,
    }).then(res => {
      this.setState({creating: false});
      if (res.status === "error") {
        Setting.showMessage("error", `${i18next.t("token:Failed to add")}: ${res.msg}`);
      } else {
        if (res.data && res.data.secretKey) {
          this.setState({createdSecretKey: res.data.secretKey});
        } else {
          this.closeCreateModal();
          Setting.showMessage("success", i18next.t("token:Token added successfully"));
        }
        this.fetch({pagination: this.state.pagination});
      }
    }).catch(error => {
      this.setState({creating: false});
      Setting.showMessage("error", `${i18next.t("token:Failed to add")}: ${error}`);
    });
  }

  copySecretKey() {
    navigator.clipboard.writeText(this.state.createdSecretKey).then(() => {
      Setting.showMessage("success", i18next.t("token:Secret key copied"));
    }).catch(() => {
      Setting.showMessage("error", i18next.t("token:Failed to copy"));
    });
  }

  renderStatusTag(status) {
    return status === "enabled" ? (
      <Tag icon={<CheckCircleOutlined />} color="success">{i18next.t("general:Enabled")}</Tag>
    ) : (
      <Tag icon={<CloseCircleOutlined />} color="error">{i18next.t("general:Disabled")}</Tag>
    );
  }

  renderCreateModal() {
    const hasModels = this.state.channelModels.length > 0;
    const createdSecretKey = this.state.createdSecretKey;
    return (
      <Modal
        title={i18next.t("token:New Token")}
        visible={this.state.createModalVisible}
        onOk={this.addToken.bind(this)}
        onCancel={this.closeCreateModal.bind(this)}
        confirmLoading={this.state.creating}
        okText={i18next.t("token:Create")}
        cancelText={i18next.t("general:Cancel")}
        footer={
          createdSecretKey !== "" ? (
            <Button type="primary" onClick={this.closeCreateModal.bind(this)}>
              {i18next.t("token:Close")}
            </Button>
          ) : undefined
        }
      >
        {
          createdSecretKey !== "" ? (
            <div>
              <Alert
                type="warning"
                showIcon
                message={i18next.t("token:This secret key will only be shown once. Please save it now")}
              />
              <pre style={{background: "#f5f5f5", padding: "12px", borderRadius: "4px", wordBreak: "break-all", marginTop: "12px"}}>
                {createdSecretKey}
              </pre>
              <Button
                type="primary"
                icon={<CopyOutlined />}
                onClick={this.copySecretKey.bind(this)}
                style={{marginTop: "8px"}}
              >
                {i18next.t("token:Copy Secret Key")}
              </Button>
            </div>
          ) : (
            <div>
              {
                !hasModels ? (
                  <Alert
                    type="warning"
                    showIcon
                    message={i18next.t("token:No channels yet. Please create a channel first")}
                    style={{marginBottom: "12px"}}
                  />
                ) : null
              }
              <p style={{marginBottom: "4px"}}>{i18next.t("general:Name")}:</p>
              <Input
                value={this.state.newTokenName}
                onChange={e => {
                  this.setState({newTokenName: e.target.value});
                }}
                placeholder={i18next.t("token:Token name placeholder")}
              />
              <p style={{margin: "12px 0 4px 0"}}>{i18next.t("token:Allowed Models")}:</p>
              <Select
                mode="multiple"
                style={{width: "100%"}}
                placeholder={i18next.t("token:Allowed Models hint")}
                value={this.state.newTokenModels}
                onChange={value => {
                  this.setState({newTokenModels: value});
                }}
              >
                {
                  this.state.channelModels.map(model => (
                    <Option key={model} value={model}>{model}</Option>
                  ))
                }
              </Select>
            </div>
          )
        }
      </Modal>
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
        sorter: true,
        render: (text, record) => {
          return (
            <Tooltip title={text}>
              <Link to={`/tokens/${record.owner}/${record.name}`}>
                {text}
              </Link>
            </Tooltip>
          );
        },
      },
      {
        title: i18next.t("token:Secret Key"),
        dataIndex: "secretKeyPrefix",
        key: "secretKeyPrefix",
        width: "140px",
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
        title: i18next.t("token:Allowed Models"),
        dataIndex: "allowedModels",
        key: "allowedModels",
        width: "180px",
        render: (models) => {
          if (!models || models.length === 0) {return "-";}
          return models.map(model => {
            return <Tag key={model} color="blue">{model}</Tag>;
          });
        },
      },
      {
        title: i18next.t("token:Rate Limit"),
        dataIndex: "rateLimit",
        key: "rateLimit",
        width: "110px",
        sorter: true,
        render: (val) => val > 0 ? `${val}/min` : i18next.t("token:Unlimited"),
      },
      {
        title: i18next.t("token:Expire Time"),
        dataIndex: "expireTime",
        key: "expireTime",
        width: "160px",
        ellipsis: true,
        sorter: true,
        render: (text) => {
          if (!text) {return "-";}
          return text;
        },
      },
      {
        title: i18next.t("general:Status"),
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
                onClick={() => this.props.history.push(`/tokens/${record.owner}/${record.name}`)}
              >
                {i18next.t("general:Edit")}
              </Button>
              <Popconfirm
                title={`${i18next.t("general:Delete")} token "${record.name}"?`}
                onConfirm={() => this.deleteToken(record)}
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
          className="token-table"
          columns={columns}
          dataSource={data}
          rowKey={(record) => `${record.owner}/${record.name}`}
          bordered
          pagination={this.state.pagination}
          title={() => (
            <div>
              {i18next.t("token:Tokens")}&nbsp;&nbsp;&nbsp;&nbsp;
              <Button type="primary" size="small" icon={<PlusOutlined />} onClick={this.openCreateModal.bind(this)}>
                {i18next.t("token:New Token")}
              </Button>
            </div>
          )}
          loading={this.state.loading}
          onChange={this.handleTableChange}
        />
        {this.renderCreateModal()}
      </div>
    );
  }
}

export default TokenListPage;
