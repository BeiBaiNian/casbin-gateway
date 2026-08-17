// Copyright 2026 The casbin Authors. All Rights Reserved.
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
import {Avatar, Button, Card, Col, Form, Input, Modal, Row} from "antd";
import i18next from "i18next";
import * as AccountBackend from "./backend/AccountBackend";
import * as Setting from "./Setting";

// Profile and password editing for the built-in login. Casdoor-backed accounts
// are managed in Casdoor itself, so this page is only reachable without Casdoor.
class AccountPage extends React.Component {
  constructor(props) {
    super(props);
    this.formRef = React.createRef();
    this.state = {
      avatar: props.account?.avatar ?? "",
      passwordModalVisible: false,
      currentPassword: "",
      newPassword: "",
    };
  }

  onFinish(values) {
    AccountBackend.updateAccount(values)
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", "Successfully saved");
          window.location.reload();
        } else {
          Setting.showMessage("error", res.msg);
        }
      })
      .catch((error) => Setting.showMessage("error", error.message));
  }

  closePasswordModal() {
    this.setState({
      passwordModalVisible: false,
      currentPassword: "",
      newPassword: "",
    });
  }

  setPassword() {
    const values = this.formRef.current.getFieldsValue();
    AccountBackend.updateAccount({...values, currentPassword: this.state.currentPassword, newPassword: this.state.newPassword})
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", "Successfully saved");
          this.closePasswordModal();
        } else {
          Setting.showMessage("error", res.msg);
        }
      })
      .catch((error) => Setting.showMessage("error", error.message));
  }

  renderAvatar() {
    const account = this.props.account;
    const avatarUrl = this.state.avatar || "";

    if (!avatarUrl.startsWith("http://") && !avatarUrl.startsWith("https://") && !avatarUrl.startsWith("data:image/")) {
      return (
        <Avatar style={{backgroundColor: Setting.getAvatarColor(account.name)}} size={64}>
          {Setting.getShortName(account.name)}
        </Avatar>
      );
    }

    return (
      <Avatar src={avatarUrl} size={64}>
        {Setting.getShortName(account.name)}
      </Avatar>
    );
  }

  renderField(label, control, span = 12) {
    return (
      <Col style={{marginTop: "12px"}} span={Setting.isMobile() ? 24 : span}>
        <div style={{marginBottom: "6px", fontWeight: 500}}>{label}</div>
        {control}
      </Col>
    );
  }

  render() {
    const account = this.props.account;

    return (
      <div style={{padding: "16px 20px 32px"}}>
        <div style={{marginBottom: "16px", display: "flex", justifyContent: "space-between", alignItems: "center"}}>
          <span style={{fontSize: "22px", fontWeight: 600}}>{i18next.t("account:My Account")}</span>
          <Button type="primary" onClick={() => this.formRef.current.submit()}>{i18next.t("general:Save")}</Button>
        </div>

        <Form
          ref={this.formRef}
          initialValues={{
            username: account.name,
            displayName: account.displayName,
            avatar: account.avatar,
          }}
          onFinish={(values) => this.onFinish(values)}
          onValuesChange={(_, values) => this.setState({avatar: values.avatar ?? ""})}
        >
          <Card size="small" title={i18next.t("account:Profile")} style={{marginBottom: "16px"}}>
            <Row gutter={[16, 8]}>
              {this.renderField(
                i18next.t("general:Name"),
                <Form.Item name="username" style={{margin: 0}}>
                  <Input disabled />
                </Form.Item>
              )}
              {this.renderField(
                i18next.t("general:Display name"),
                <Form.Item name="displayName" style={{margin: 0}}>
                  <Input />
                </Form.Item>
              )}
              {this.renderField(
                i18next.t("account:Avatar"),
                <Row gutter={10} align="middle">
                  <Col flex="80px">
                    {this.renderAvatar()}
                  </Col>
                  <Col flex="auto">
                    <Form.Item name="avatar" style={{margin: 0}}>
                      <Input placeholder={i18next.t("account:Avatar image URL, optional")} />
                    </Form.Item>
                  </Col>
                </Row>,
                24
              )}
            </Row>
          </Card>

          <Card size="small" title={i18next.t("general:Password")}>
            <Button onClick={() => this.setState({passwordModalVisible: true})}>
              {i18next.t("account:Modify password...")}
            </Button>
          </Card>
        </Form>

        <Modal
          maskClosable={false}
          title={i18next.t("account:Modify password")}
          open={this.state.passwordModalVisible}
          okText={i18next.t("account:Set Password")}
          cancelText={i18next.t("general:Cancel")}
          onCancel={() => this.closePasswordModal()}
          onOk={() => this.setPassword()}
          width={520}
        >
          <div style={{padding: "16px 0 8px"}}>
            <div style={{marginBottom: "6px", fontWeight: 500}}>{i18next.t("account:Old Password")}</div>
            <Input.Password
              style={{marginBottom: "20px"}}
              value={this.state.currentPassword}
              onChange={e => this.setState({currentPassword: e.target.value})}
            />
            <div style={{marginBottom: "6px", fontWeight: 500}}>{i18next.t("account:New Password")}</div>
            <Input.Password
              value={this.state.newPassword}
              onChange={e => this.setState({newPassword: e.target.value})}
            />
          </div>
        </Modal>
      </div>
    );
  }
}

export default AccountPage;
