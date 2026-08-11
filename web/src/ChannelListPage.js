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
import {Button, Popconfirm, Table, Tag} from "antd";
import BaseListPage from "./BaseListPage";
import * as ChannelBackend from "./backend/ChannelBackend";
import * as Setting from "./Setting";

class ChannelListPage extends BaseListPage {
  UNSAFE_componentWillMount() {
    this.fetch = this.fetchChannels;
    this.fetchChannels();
  }

  fetchChannels = () => {
    ChannelBackend.getChannels(this.props.account.name).then(res => {
      if (res.status === "ok") {
        this.setState({
          data: res.data,
        });
      }
    });
  };

  deleteChannel = (channel) => {
    ChannelBackend.deleteChannel(channel).then(() => {
      this.fetchChannels();
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
        Setting.showMessage("error", `Failed to add: ${res.msg}`);
      } else {
        Setting.showMessage("success", "Channel added successfully");
        this.fetchChannels();
      }
    }).catch(error => {
      Setting.showMessage("error", `Channel failed to add: ${error}`);
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
        title: "Type",
        dataIndex: "type",
        key: "type",
      },
      {
        title: "BaseUrl",
        dataIndex: "baseUrl",
        key: "baseUrl",
      },
      {
        title: "Models",
        dataIndex: "models",
        key: "models",
        render: (models) => {
          return (models || []).map(model => {
            return <Tag key={model}>{model}</Tag>;
          });
        },
      },
      {
        title: "Priority",
        dataIndex: "priority",
        key: "priority",
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
              <Link to={`/channels/${record.owner}/${record.name}`}>Edit</Link>
              &nbsp;
              <Popconfirm title="Delete?" onConfirm={() => this.deleteChannel(record)}>
                <a>Delete</a>
              </Popconfirm>
            </span>
          );
        },
      },
    ];

    return (
      <div>
        <Button type="primary" onClick={this.addChannel.bind(this)}>
          New Channel
        </Button>
        <Table rowKey="name" dataSource={data} columns={columns} />
      </div>
    );
  }
}

export default ChannelListPage;
