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
import {Alert, AutoComplete, Button, Card, Col, Input, InputNumber, Result, Row, Select, Space, Spin} from "antd";
import {ApiOutlined, CheckCircleOutlined, CloseCircleOutlined, SaveOutlined, ThunderboltOutlined} from "@ant-design/icons";
import i18next from "i18next";
import * as ChannelBackend from "./backend/ChannelBackend";
import * as Setting from "./Setting";

const {Option} = Select;

// Hard-coded presets for milestone 1.1 (per channel type).
const BASE_URL_PRESETS = {
  openai: ["https://api.openai.com/v1"],
  custom: ["https://oneapi.example.com", "https://api.deepseek.com/v1", "https://api.moonshot.cn/v1"],
};

// Mirrors object.BuildOpenAiUrl on the server.
function buildOpenAiUrl(baseUrl, endpoint) {
  let url;
  try {
    url = new URL(baseUrl);
  } catch {
    return "";
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    return "";
  }

  let path = url.pathname.replace(/\/+$/, "");
  if (path.endsWith(endpoint)) {
    path = path.slice(0, path.length - endpoint.length);
  }
  if (!path.endsWith("/v1")) {
    path += "/v1";
  }

  url.pathname = path + endpoint;
  return url.toString();
}

const MODEL_PRESETS = {
  openai: ["gpt-5.5", "gpt-5", "gpt-5-mini", "o3", "o4-mini"],
  custom: ["deepseek-chat", "deepseek-reasoner", "moonshot-v1-8k", "qwen-max"],
};

class ChannelEditPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      // undefined while loading, null when the channel could not be loaded.
      channel: undefined,
      testing: false,
      result: null,
    };
  }

  componentDidMount() {
    this.load();
  }

  load() {
    const {owner, channelName} = this.props.match.params;
    ChannelBackend.getChannel(owner, channelName).then(res => {
      if (res.status === "ok") {
        this.setState({channel: res.data});
      } else {
        this.setState({channel: null});
        Setting.showMessage("error", `${i18next.t("channel:Failed to get channel")}: ${res.msg}`);
      }
    }).catch(error => {
      this.setState({channel: null});
      Setting.showMessage("error", `${i18next.t("channel:Failed to get channel")}: ${error}`);
    });
  }

  setChannelField(key, value) {
    this.setState({
      channel: {
        ...this.state.channel,
        [key]: value,
      },
    });
  }

  save() {
    const {owner, channelName} = this.props.match.params;
    ChannelBackend.updateChannel(owner, channelName, this.state.channel).then(res => {
      if (res.status === "error") {
        Setting.showMessage("error", `${i18next.t("channel:Failed to save")}: ${res.msg}`);
        return;
      }

      Setting.showMessage("success", i18next.t("channel:Channel saved"));
    }).catch(error => {
      Setting.showMessage("error", `${i18next.t("channel:Failed to save")}: ${error}`);
    });
  }

  test() {
    const {owner, channelName} = this.props.match.params;
    this.setState({testing: true, result: null});

    ChannelBackend.testChannel(owner, channelName).then(res => {
      if (res.status === "error") {
        this.setState({testing: false});
        Setting.showMessage("error", `${i18next.t("channel:Failed to test")}: ${res.msg}`);
        return;
      }

      this.setState({testing: false, result: res.data});
    }).catch(error => {
      this.setState({testing: false});
      Setting.showMessage("error", `${i18next.t("channel:Failed to test")}: ${error}`);
    });
  }

  renderUpstreamUrl(channel) {
    const upstreamUrl = buildOpenAiUrl(channel.baseUrl, "/chat/completions");
    if (upstreamUrl === "") {
      return null;
    }

    return (
      <div style={{color: "#888", fontSize: "12px", marginTop: "4px"}}>
        {i18next.t("channel:Base URL hint")}: <span style={{fontFamily: "monospace"}}>{upstreamUrl}</span>
      </div>
    );
  }

  render() {
    const channel = this.state.channel;
    if (channel === undefined) {
      return (
        <div style={{textAlign: "center", marginTop: "100px"}}>
          <Spin size="large" />
        </div>
      );
    }
    if (channel === null) {
      return (
        <Result
          status="404"
          title={i18next.t("channel:Channel not found")}
          extra={<Button type="primary" onClick={() => this.props.history.push("/channels")}>{i18next.t("channel:Channels")}</Button>}
        />
      );
    }

    return (
      <Card
        title={
          <span>
            <ApiOutlined style={{marginRight: "8px", color: "#1890ff"}} />
            {i18next.t("channel:Edit Channel")}: {channel.displayName}
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
            <Button
              icon={<ThunderboltOutlined />}
              loading={this.state.testing}
              onClick={this.test.bind(this)}
            >
              {i18next.t("channel:Test Connectivity")}
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
              value={channel.displayName}
              onChange={e => {
                this.setChannelField("displayName", e.target.value);
              }}
            />
          </Col>
        </Row>

        <Row style={{marginTop: "20px"}}>
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("channel:Type")}:
          </Col>
          <Col span={22}>
            <Select
              style={{width: "100%"}}
              value={channel.type}
              onChange={value => {
                this.setChannelField("type", value);
              }}
            >
              <Option value="openai">OpenAI</Option>
              <Option value="custom">Custom</Option>
            </Select>
          </Col>
        </Row>

        <Row style={{marginTop: "20px"}}>
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("channel:Base URL")}:
          </Col>
          <Col span={22}>
            <AutoComplete
              style={{width: "100%"}}
              value={channel.baseUrl}
              options={(BASE_URL_PRESETS[channel.type] || []).map(url => ({value: url, label: url}))}
              onChange={value => {
                this.setChannelField("baseUrl", value);
              }}
              placeholder="https://api.openai.com/v1"
            />
            {this.renderUpstreamUrl(channel)}
          </Col>
        </Row>

        <Row style={{marginTop: "20px"}}>
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("channel:API Key")}:
          </Col>
          <Col span={22}>
            <Input.Password
              placeholder="sk-..."
              value={channel.apiKey}
              onChange={e => {
                this.setChannelField("apiKey", e.target.value);
              }}
            />
            <div style={{color: "#888", fontSize: "12px", marginTop: "4px"}}>
              {i18next.t("channel:API Key hint")}
            </div>
          </Col>
        </Row>

        <Row style={{marginTop: "20px"}}>
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("channel:Models")}:
          </Col>
          <Col span={22}>
            <Select
              mode="tags"
              style={{width: "100%"}}
              placeholder="gpt-5, gpt-5-mini"
              value={channel.models}
              onChange={value => {
                this.setChannelField("models", value);
              }}
            >
              {
                (MODEL_PRESETS[channel.type] || []).map(model => (
                  <Option key={model} value={model}>{model}</Option>
                ))
              }
            </Select>
          </Col>
        </Row>

        <Row style={{marginTop: "20px"}}>
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("channel:Priority")}:
          </Col>
          <Col span={22}>
            <InputNumber
              min={0}
              value={channel.priority}
              onChange={value => {
                this.setChannelField("priority", value);
              }}
            />
            <div style={{color: "#888", fontSize: "12px", marginTop: "4px"}}>
              {i18next.t("channel:Priority hint")}
            </div>
          </Col>
        </Row>

        <Row style={{marginTop: "20px"}}>
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("channel:Status")}:
          </Col>
          <Col span={22}>
            <Select
              style={{width: "100%"}}
              value={channel.status}
              onChange={value => {
                this.setChannelField("status", value);
              }}
            >
              <Option value="enabled">
                <CheckCircleOutlined style={{color: "#52c41a", marginRight: "8px"}} />
                {i18next.t("channel:Enabled")}
              </Option>
              <Option value="disabled">
                <CloseCircleOutlined style={{color: "#ff4d4f", marginRight: "8px"}} />
                {i18next.t("channel:Disabled")}
              </Option>
            </Select>
          </Col>
        </Row>

        {
          this.state.result ? (
            <Row style={{marginTop: "20px"}}>
              <Col span={2} />
              <Col span={22}>
                <Alert
                  message={this.state.result.success ? i18next.t("channel:Connection Successful") : i18next.t("channel:Connection Failed")}
                  description={this.state.result.statusCode ? `HTTP ${this.state.result.statusCode} - ${this.state.result.message}` : this.state.result.message}
                  type={this.state.result.success ? "success" : "error"}
                  showIcon
                />
              </Col>
            </Row>
          ) : null
        }
      </Card>
    );
  }
}

export default ChannelEditPage;
