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

import ReactECharts from "echarts-for-react";
import i18next from "i18next";

import {Card, CardContent, CardHeader, CardTitle} from "@/components/ui/card";
import type {MetricPoint} from "@/types";

export function BarChartCard({title, data}: {title: string; data: MetricPoint[] | null}) {
  const points = data ?? [];
  const option = {
    tooltip: {trigger: "axis", axisPointer: {type: "shadow"}},
    grid: {left: "3%", right: "4%", bottom: "3%", containLabel: true},
    xAxis: {
      type: "category",
      data: points.map(item => item.data),
      axisTick: {alignWithLabel: true},
    },
    yAxis: {type: "value"},
    series: [
      {
        name: title,
        type: "bar",
        barWidth: "60%",
        data: points.map(item => item.count),
      },
    ],
  };

  return (
    <Card className="h-full">
      <CardHeader className="p-4 pb-0">
        <CardTitle className="text-sm">{i18next.t(`general:${title}`)}</CardTitle>
      </CardHeader>
      <CardContent className="p-2">
        <ReactECharts option={option} style={{height: 300}} />
      </CardContent>
    </Card>
  );
}

export function PieChartCard({
  title,
  data,
}: {
  title: string;
  data: {name: string; value: number}[] | null;
}) {
  const option = {
    tooltip: {trigger: "item"},
    legend: {top: "5%", left: "right", orient: "vertical"},
    series: [
      {
        name: title,
        type: "pie",
        radius: ["40%", "70%"],
        avoidLabelOverlap: false,
        itemStyle: {borderRadius: 10, borderColor: "#fff", borderWidth: 2},
        label: {show: false, position: "center"},
        emphasis: {label: {show: true, fontSize: 24, fontWeight: "bold"}},
        labelLine: {show: false},
        data: data ?? [],
      },
    ],
  };

  return (
    <Card className="h-full">
      <CardHeader className="p-4 pb-0">
        <CardTitle className="text-sm">{i18next.t(`general:${title}`)}</CardTitle>
      </CardHeader>
      <CardContent className="p-2">
        <ReactECharts option={option} style={{height: 300}} />
      </CardContent>
    </Card>
  );
}

/**
 * StatisticCard shows one big number. The antd build drew it with an echarts
 * scatter plot; plain markup renders the same thing without a chart runtime.
 */
export function StatisticCard({title, value}: {title: string; value: number}) {
  return (
    <Card className="flex h-full flex-col">
      <CardHeader className="p-4 pb-0">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-1 items-center justify-center p-4">
        <span className="text-4xl font-semibold tabular-nums">{value.toLocaleString()}</span>
      </CardContent>
    </Card>
  );
}
