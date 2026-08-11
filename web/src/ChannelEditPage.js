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
import {Alert, Button, Card, Input, InputNumber, Select} from "antd";
import * as ChannelBackend from "./backend/ChannelBackend";
import * as Setting from "./Setting";

const {Option} = Select;

class ChannelEditPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      channel: null,
      loading: false,
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
        this.setState({
          channel: res.data,
        });
      }
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
    ChannelBackend.updateChannel(owner, channelName, this.state.channel).then(() => {
      Setting.showMessage("success", "Channel saved");
    });
  }

  test() {
    const {owner, channelName} = this.props.match.params;
    this.setState({
      loading: true,
    });
    ChannelBackend.testChannel(owner, channelName).then(res => {
      this.setState({
        loading: false,
        result: res.data,
      });
    });
  }

  render() {
    const channel = this.state.channel;
    if (!channel) {
      return null;
    }

    return (
      <Card title={<Button type="primary" onClick={this.save.bind(this)}>Save</Button>}>
        <Input addonBefore="DisplayName" value={channel.displayName} onChange={e => {
          this.setChannelField("displayName", e.target.value);
        }} />
        <Select style={{width: "100%"}} value={channel.type} onChange={value => {
          this.setChannelField("type", value);
        }}>
          <Option value="openai">openai</Option>
          <Option value="claude">claude</Option>
          <Option value="gemini">gemini</Option>
          <Option value="custom">custom</Option>
        </Select>
        <Input addonBefore="BaseUrl" value={channel.baseUrl} onChange={e => {
          this.setChannelField("baseUrl", e.target.value);
        }} />
        <Input.Password addonBefore="ApiKey" value={channel.apiKey} onChange={e => {
          this.setChannelField("apiKey", e.target.value);
        }} />
        <Select mode="tags" style={{width: "100%"}} value={channel.models} onChange={value => {
          this.setChannelField("models", value);
        }} />
        <InputNumber value={channel.priority} onChange={value => {
          this.setChannelField("priority", value);
        }} />
        <Select value={channel.status} onChange={value => {
          this.setChannelField("status", value);
        }}>
          <Option value="enabled">enabled</Option>
          <Option value="disabled">disabled</Option>
        </Select>
        <Button loading={this.state.loading} onClick={this.test.bind(this)}>Test Connectivity</Button>
        {
          this.state.result ? (
            <Alert message={this.state.result.message} type={this.state.result.success ? "success" : "error"} />
          ) : null
        }
      </Card>
    );
  }
}

export default ChannelEditPage;
