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
import {Button, Modal, Popconfirm, Table, Tag} from "antd";
import BaseListPage from "./BaseListPage";
import * as TokenBackend from "./backend/TokenBackend";
import * as Setting from "./Setting";

class TokenListPage extends BaseListPage {
  UNSAFE_componentWillMount() {
    this.fetch = this.fetchTokens;
    this.fetchTokens();
  }

  fetchTokens = () => {
    TokenBackend.getTokens(this.props.account.name).then(res => {
      if (res.status === "ok") {
        this.setState({
          data: res.data,
        });
      }
    });
  };

  deleteToken = (token) => {
    TokenBackend.deleteToken(token).then(() => {
      this.fetchTokens();
    });
  };

  newToken() {
    const randomName = Setting.getRandomName();
    return {
      owner: this.props.account.name,
      name: `token_${randomName}`,
      displayName: `New Token - ${randomName}`,
      status: "enabled",
      allowedModels: [],
      rateLimit: 0,
    };
  }

  addToken() {
    const newToken = this.newToken();
    TokenBackend.addToken(newToken).then(res => {
      if (res.status === "error") {
        Setting.showMessage("error", `Failed to add: ${res.msg}`);
      } else {
        Setting.showMessage("success", "Token added successfully");
        if (res.data && res.data.secretKey) {
          Modal.info({
            title: "Token Secret Key",
            content: (
              <div>
                <p>This secret key will only be shown once. Please save it now:</p>
                <pre style={{background: "#f5f5f5", padding: "12px", borderRadius: "4px", wordBreak: "break-all"}}>
                  {res.data.secretKey}
                </pre>
              </div>
            ),
            width: 600,
          });
        }
        this.fetchTokens();
      }
    }).catch(error => {
      Setting.showMessage("error", `Token failed to add: ${error}`);
    });
  }

  renderTable(data) {
    const columns = [
      {
        title: "DisplayName",
        dataIndex: "displayName",
        key: "displayName",
      },
      {
        title: "SecretKey",
        dataIndex: "secretKeyPrefix",
        key: "secretKeyPrefix",
      },
      {
        title: "AllowedModels",
        dataIndex: "allowedModels",
        key: "allowedModels",
        render: (models) => {
          return (models || []).map(model => {
            return <Tag key={model}>{model}</Tag>;
          });
        },
      },
      {
        title: "RateLimit",
        dataIndex: "rateLimit",
        key: "rateLimit",
        render: (val) => val > 0 ? `${val}/min` : "Unlimited",
      },
      {
        title: "ExpireTime",
        dataIndex: "expireTime",
        key: "expireTime",
      },
      {
        title: "Status",
        dataIndex: "status",
        key: "status",
      },
      {
        title: "Operation",
        key: "operation",
        render: (text, record) => {
          return (
            <span>
              <Link to={`/tokens/${record.owner}/${record.name}`}>Edit</Link>
              &nbsp;
              <Popconfirm title="Delete?" onConfirm={() => this.deleteToken(record)}>
                <a>Delete</a>
              </Popconfirm>
            </span>
          );
        },
      },
    ];

    return (
      <div>
        <Button type="primary" onClick={this.addToken.bind(this)}>
          New Token
        </Button>
        <Table rowKey="name" dataSource={data} columns={columns} />
      </div>
    );
  }
}

export default TokenListPage;
