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
import {Button, Form, Input, Result, Spin} from "antd";
import {LockOutlined, UserOutlined} from "@ant-design/icons";
import i18next from "i18next";
import * as AccountBackend from "./backend/AccountBackend";
import * as Conf from "./Conf";
import * as Setting from "./Setting";

// Signing in goes one of two ways: redirect to Casdoor when app.conf configures
// it, otherwise show the built-in username/password form.
class SigninPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      loading: true,
      showSignin: false,
      errorMessage: "",
      autoSignin: false,
    };
  }

  componentDidMount() {
    AccountBackend.getSigninOptions()
      .then((res) => {
        if (res.status === "ok" && res.data?.casdoorAvailable) {
          // Do not wait for App to publish the config: this page may well have
          // mounted before that request came back.
          Conf.setAuthConfig(res.data.authConfig);
          Setting.initCasdoorSdk(Conf.AuthConfig);
          window.location.replace(Setting.getSigninUrl());
          return;
        }

        this.setState({
          loading: false,
          showSignin: res.status === "ok" && res.data?.signinAvailable,
          errorMessage: res.status === "ok" ? "" : res.msg,
          autoSignin: res.status === "ok" && res.data?.autoSignin === true,
        });
      })
      .catch((error) => {
        this.setState({
          loading: false,
          showSignin: false,
          errorMessage: error.message,
        });
      });
  }

  onFinish(values) {
    AccountBackend.signinWithPassword(values.username, values.password)
      .then((res) => {
        if (res.status === "ok") {
          const from = sessionStorage.getItem("from") || "/";
          sessionStorage.removeItem("from");
          window.location.href = from;
        } else {
          Setting.showMessage("error", res.msg);
        }
      })
      .catch((error) => Setting.showMessage("error", error.message));
  }

  render() {
    if (this.state.loading) {
      return (
        <div style={{display: "flex", alignItems: "center", justifyContent: "center", minHeight: "60vh"}}>
          <Spin size="large" tip={"Signing in..."} />
        </div>
      );
    }

    if (!this.state.showSignin) {
      return (
        <Result
          status="warning"
          title={"Login Error"}
          subTitle={this.state.errorMessage || i18next.t("account:Sign in is unavailable")}
        />
      );
    }

    return (
      <div style={{display: "flex", alignItems: "center", justifyContent: "center", minHeight: "60vh"}}>
        <div style={{width: "340px"}}>
          <div style={{textAlign: "center", marginBottom: "36px", fontSize: "22px", fontWeight: 600}}>
            {i18next.t("account:Sign In")}
          </div>
          <Form initialValues={{username: "admin", password: this.state.autoSignin ? "123" : undefined}} onFinish={(values) => this.onFinish(values)} requiredMark={false}>
            <Form.Item name="username" rules={[{required: true, message: i18next.t("account:Please input your username")}]}>
              <Input
                prefix={<UserOutlined />}
                placeholder={i18next.t("account:Username")}
                style={{height: "42px"}}
              />
            </Form.Item>
            <Form.Item name="password" rules={[{required: true, message: i18next.t("account:Please input your password")}]}>
              <Input.Password
                prefix={<LockOutlined />}
                placeholder={i18next.t("general:Password")}
                autoFocus
                style={{height: "42px"}}
              />
            </Form.Item>
            <Button type="primary" htmlType="submit" block style={{height: "42px", marginTop: "8px"}}>
              {i18next.t("account:Sign In")}
            </Button>
          </Form>
        </div>
      </div>
    );
  }
}

export default SigninPage;
