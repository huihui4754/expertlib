// Copyright 2025 ipanel Inc (authors: shenghui zhang)

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import "sync"

// 存储对话相关信息的结构体
type DialogInfo struct {
	UserID      string       `json:"user_id"`              // 自建平台认证用户id
	UserAgent   string       `json:"user_agent,omitempty"` // user_agent 是客户端的标识，用于区分不同的客户端
	Platform    string       `json:"platform,omitempty"`   // platform 是客户端平台的标识 如 lark ,web ,speaker,等
	DialogID    string       `json:"dialog_id"`            // 目前的对话ID
	Program     string       `json:"program"`              // 目前对接的程序库，为空字符串代表现在没对接专家,需要小壮分析用户需求来分配一个程序库，
	Mutil       bool         `json:"Mutil"`                // 多轮对话控制,是否在多轮对话
	FirstMutil  bool         `json:"FirstMutil"`           // 是否是多轮对话的第一句话
	ChatHistory []string     `json:"chat_history"`         // 当前dialog的历史消息记录
	RWMutex     sync.RWMutex `json:"-"`
}
