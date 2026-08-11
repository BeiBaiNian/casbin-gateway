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
import {Button, Card, DatePicker, Input, InputNumber, Select} from "antd";
import moment from "moment";
import * as TokenBackend from "./backend/TokenBackend";
import * as Setting from "./Setting";

const {Option} = Select;

class TokenEditPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      token: null,
    };
  }

  componentDidMount() {
    this.load();
  }

  load() {
    const {owner, tokenName} = this.props.match.params;
    TokenBackend.getToken(owner, tokenName).then(res => {
      if (res.status === "ok") {
        this.setState({
          token: res.data,
        });
      }
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
    TokenBackend.updateToken(owner, tokenName, this.state.token).then(() => {
      Setting.showMessage("success", "Token saved");
    });
  }

  render() {
    const token = this.state.token;
    if (!token) {
      return null;
    }

    return (
      <Card title={<Button type="primary" onClick={this.save.bind(this)}>Save</Button>}>
        <Input addonBefore="DisplayName" value={token.displayName} onChange={e => {
          this.setTokenField("displayName", e.target.value);
        }} />
        <Select mode="tags" style={{width: "100%"}} value={token.allowedModels} onChange={value => {
          this.setTokenField("allowedModels", value);
        }} placeholder="Allowed Models (empty = all)" />
        <InputNumber addonBefore="RateLimit (/min)" value={token.rateLimit} onChange={value => {
          this.setTokenField("rateLimit", value || 0);
        }} min={0} style={{width: "100%"}} />
        <DatePicker showTime value={token.expireTime ? moment(token.expireTime) : null} onChange={(date, dateString) => {
          this.setTokenField("expireTime", dateString || "");
        }} style={{width: "100%"}} placeholder="Expire Time (empty = never)" />
        <Select value={token.status} onChange={value => {
          this.setTokenField("status", value);
        }} style={{width: "100%"}}>
          <Option value="enabled">enabled</Option>
          <Option value="disabled">disabled</Option>
        </Select>
      </Card>
    );
  }
}

export default TokenEditPage;
