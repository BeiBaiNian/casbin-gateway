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
import {Button, Card, Col, DatePicker, Input, InputNumber, Result, Row, Select, Space, Spin} from "antd";
import {CheckCircleOutlined, CloseCircleOutlined, KeyOutlined, SaveOutlined} from "@ant-design/icons";
import i18next from "i18next";
import moment from "moment";
import * as ChannelBackend from "./backend/ChannelBackend";
import * as TokenBackend from "./backend/TokenBackend";
import * as Setting from "./Setting";

const {Option} = Select;

class TokenEditPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      token: undefined,
      channelModels: [],
    };
  }

  componentDidMount() {
    this.load();
    this.fetchChannelModels();
  }

  fetchChannelModels() {
    const {owner} = this.props.match.params;
    ChannelBackend.getChannels(owner, 1, 1000).then(res => {
      if (res.status === "ok") {
        const modelSet = new Set();
        res.data.forEach(channel => {
          (channel.models || []).forEach(model => modelSet.add(model));
        });
        this.setState({channelModels: Array.from(modelSet).sort()});
      }
    });
  }

  load() {
    const {owner, tokenName} = this.props.match.params;
    TokenBackend.getToken(owner, tokenName).then(res => {
      if (res.status === "ok") {
        this.setState({token: res.data});
      } else {
        this.setState({token: null});
        Setting.showMessage("error", `${i18next.t("token:Failed to get token")}: ${res.msg}`);
      }
    }).catch(error => {
      this.setState({token: null});
      Setting.showMessage("error", `${i18next.t("token:Failed to get token")}: ${error}`);
    });
  }

  setTokenField(key, value) {
    this.setState({
      token: {
        ...this.state.token,
        [key]: value,
      },
    });
  }

  save() {
    const {owner, tokenName} = this.props.match.params;
    TokenBackend.updateToken(owner, tokenName, this.state.token).then(res => {
      if (res.status === "error") {
        Setting.showMessage("error", `${i18next.t("token:Failed to save")}: ${res.msg}`);
        return;
      }

      Setting.showMessage("success", i18next.t("token:Token saved"));
    }).catch(error => {
      Setting.showMessage("error", `${i18next.t("token:Failed to save")}: ${error}`);
    });
  }

  render() {
    const token = this.state.token;
    if (token === undefined) {
      return (
        <div style={{textAlign: "center", marginTop: "100px"}}>
          <Spin size="large" />
        </div>
      );
    }
    if (token === null) {
      return (
        <Result
          status="404"
          title={i18next.t("token:Token not found")}
          extra={<Button type="primary" onClick={() => this.props.history.push("/tokens")}>{i18next.t("token:Tokens")}</Button>}
        />
      );
    }

    return (
      <Card
        title={
          <span>
            <KeyOutlined style={{marginRight: "8px", color: "#1890ff"}} />
            {i18next.t("token:Edit Token")}: {token.displayName}
          </span>
        }
        extra={
          <Space>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              onClick={this.save.bind(this)}
            >
              {i18next.t("general:Save")}
            </Button>
          </Space>
        }
      >
        <Row style={{marginTop: "10px"}}>
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("general:Display name")}:
          </Col>
          <Col span={22}>
            <Input
              value={token.displayName}
              onChange={e => {
                this.setTokenField("displayName", e.target.value);
              }}
            />
          </Col>
        </Row>

        <Row style={{marginTop: "20px"}}>
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("token:Allowed Models")}:
          </Col>
          <Col span={22}>
            <Select
              mode="multiple"
              style={{width: "100%"}}
              placeholder={i18next.t("token:Allowed Models hint")}
              value={token.allowedModels}
              onChange={value => {
                this.setTokenField("allowedModels", value);
              }}
            >
              {
                this.state.channelModels.map(model => (
                  <Option key={model} value={model}>{model}</Option>
                ))
              }
            </Select>
            <div style={{color: "#888", fontSize: "12px", marginTop: "4px"}}>
              {i18next.t("token:Allowed Models hint")}
            </div>
          </Col>
        </Row>

        <Row style={{marginTop: "20px"}}>
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("token:Rate Limit")}:
          </Col>
          <Col span={22}>
            <InputNumber
              min={0}
              value={token.rateLimit}
              onChange={value => {
                this.setTokenField("rateLimit", value || 0);
              }}
              addonAfter="/min"
            />
            <div style={{color: "#888", fontSize: "12px", marginTop: "4px"}}>
              {i18next.t("token:Rate Limit hint")}
            </div>
          </Col>
        </Row>

        <Row style={{marginTop: "20px"}}>
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("token:Expire Time")}:
          </Col>
          <Col span={22}>
            <DatePicker
              showTime
              value={token.expireTime ? moment(token.expireTime) : null}
              onChange={(date) => {
                // Store RFC3339 so the backend time.Parse(time.RFC3339, ...) works.
                this.setTokenField("expireTime", date ? date.format("YYYY-MM-DDTHH:mm:ssZ") : "");
              }}
              style={{width: "100%"}}
              placeholder={i18next.t("token:Expire Time hint")}
            />
            <div style={{color: "#888", fontSize: "12px", marginTop: "4px"}}>
              {i18next.t("token:Expire Time hint")}
            </div>
          </Col>
        </Row>

        <Row style={{marginTop: "20px"}}>
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("general:Status")}:
          </Col>
          <Col span={22}>
            <Select
              style={{width: "100%"}}
              value={token.status}
              onChange={value => {
                this.setTokenField("status", value);
              }}
            >
              <Option value="enabled">
                <CheckCircleOutlined style={{color: "#52c41a", marginRight: "8px"}} />
                {i18next.t("general:Enabled")}
              </Option>
              <Option value="disabled">
                <CloseCircleOutlined style={{color: "#ff4d4f", marginRight: "8px"}} />
                {i18next.t("general:Disabled")}
              </Option>
            </Select>
          </Col>
        </Row>
      </Card>
    );
  }
}

export default TokenEditPage;
