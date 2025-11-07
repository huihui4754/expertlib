const net = require('net');
const http = require('http');
const axios = require('axios');
const crypto = require('crypto');

/**
 * 以--key=value格式解析命令行参数
 * @returns {object} 包含已分析参数的对象。
 */
function parseArgs() {
    return process.argv.slice(2).reduce((acc, arg) => {
        const [key, value] = arg.split('=');
        if (key && value) {
            acc[key.replace('--', '')] = value;
        }
        return acc;
    }, {});
}

/**
 * 根据指定的协议通过套接字创建并发送消息。
 * @param {net.Socket} socket 要将消息写入的套接字。
 * @param {number} eventType 消息的事件类型。
 * @param {object} message JSON payload。
 */
function sendMessage(socket, eventType, message) {
    const body = Buffer.from(JSON.stringify(message));
    const header = Buffer.alloc(16);

    header.writeUInt32BE(0xDEADBEEF, 0); // Magic number
    header.writeUInt16BE(1, 4);          // Protocol version
    header.writeUInt16BE(eventType, 6);  // Message type
    header.writeUInt32BE(body.length, 8);// Body length
    header.writeUInt32BE(0, 12);         // Reserved

    const fullMessage = Buffer.concat([header, body]);
    socket.write(fullMessage);
    console.log(`Sent message with event type ${eventType}`);
}

/**
 * 通过Go主机提供的HTTP接口访问内存存储。
 * @param {number} port HTTP服务器的端口。
 * @param {string} key 要查询的键。
 * @param {string} dialog_id dialog_id。
 * @returns {Promise<object>} 使用查询结果进行解析的promise。
 */
async function queryMemory(port, key, dialog_id) {
    if (!port) {
        return Promise.reject(new Error('Port not provided for memory query. Use --port=XXXX'));
    }
    const data = JSON.stringify({
        event_type: 3000,
        action: "query_tool_memory",
        key: key,
        dialog_id: dialog_id,
    });

    const options = {
        hostname: 'localhost',
        port: port,
        path: '/',
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Content-Length': data.length
        }
    };

    return new Promise((resolve, reject) => {
        const req = http.request(options, res => {
            let responseBody = '';
            res.on('data', chunk => responseBody += chunk);
            res.on('end', () => {
                if (res.statusCode === 200) {
                    try {
                        resolve(JSON.parse(responseBody));
                    } catch (e) {
                        reject(new Error('Failed to parse JSON response from memory API.'));
                    }
                } else {
                    reject(new Error(`Memory API request failed with status code ${res.statusCode}: ${responseBody}`));
                }
            });
        });

        req.on('error', error => reject(error));
        req.write(data);
        req.end();
    });
}

/**
 * 通过HTTP接口将值发送到内存存储。
 * @param {number} port HTTP服务器的端口。
 * @param {string} key 拯救的钥匙
 * @param {string} value 要保存的值。
 * @param {string} dialog_id dialog_id。
 * @returns {Promise<void>}
 */
async function saveMemory(port, key, value, dialog_id) {
    if (!port) {
        return Promise.reject(new Error('Port not provided for memory save. Use --port=XXXX'));
    }
    const data = JSON.stringify({
        event_type: 3000,
        action: "save_tool_memory",
        key: key,
        value: value,
        dialog_id: dialog_id,
    });

    const options = {
        hostname: 'localhost',
        port: port,
        path: '/',
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Content-Length': data.length
        }
    };

    return new Promise((resolve, reject) => {
        const req = http.request(options, res => {
            console.log(`Save memory status: ${res.statusCode}`);
            if (res.statusCode >= 200 && res.statusCode < 300) {
                resolve();
            } else {
                res.on('data', (chunk) => console.error(`Memory save error body: ${chunk}`));
                reject(new Error(`Save memory request failed with status code ${res.statusCode}`));
            }
        });
        req.on('error', e => reject(new Error(`Problem with save memory request: ${e.message}`)));
        req.write(data);
        req.end();
    });
}


/**
 * 主函数来运行程序。
 */
function main() {
    const args = parseArgs();
    console.log(args)
    const socketPath = args.socket;
    const port = args.port;

    if (!socketPath) {
        console.error('Error: Socket path not provided. Use --socket=/path/to/socket');
        process.exit(1);
    }

    // State management
    let waitingForConfirmation = false;
    let pendingRepoUrl = null;
    let pendingTag = null;
    let repoUrl = null;
    let tag = null;

    console.log(`Attempting to connect to socket at: ${socketPath}`);
    const socket = net.createConnection({ path: socketPath });

    socket.on('connect', () => {
        console.log('Successfully connected to Go program via socket.');
    });

    let buffer = Buffer.alloc(0);
    const HEADER_LENGTH = 16;

    /**
     * 执行实际的查询操作
     */
    async function performQuery(repoUrl, currentTag, dialog_id, user_id) {
        // await saveMemory(port, "repoUrl", repoUrl, user_id);
        // await saveMemory(port, "tag", currentTag, user_id);

        sendMessage(socket, 2001, {
            "event_type": 2001,
            "dialog_id": dialog_id,
            "user_id": user_id,
            "intention": "checkAutoStatus",
            "message_id": crypto.randomUUID(),
            "messages": {
                "content": "马上帮你查询，请稍候",
                "attachments": []
            }
        });

        const apiUrl = `${repoUrl}/build/get_auto_info/${currentTag}`;
        console.log(`API URL: ${apiUrl}`);
        try {
            const response = await axios.get(apiUrl, {
                headers: {
                    "Authorization": "Basic xxxxx"  // 改成自己的
                }
            });
            const data = response.data;

            console.log('API response data:', data);

            if (data.error_code === 0) {
                const autoInfo = data.data;
                const reply = `查询 ${repoUrl} 的 ${currentTag} 成功:\n- Auto名称: ${autoInfo['auto名称']}\n- Buildee名称: ${autoInfo['buildee名称']}\n- Auto启动时间: ${autoInfo['auto启动时间']}\n- 健康状况: ${autoInfo['健康状况']}\n- 健康持续时长: ${autoInfo['健康持续时长']}\n- 健康开始时间: ${autoInfo['健康开始时间']}`;
                sendMessage(socket, 2001, {
                    "event_type": 2001,
                    "dialog_id": dialog_id,
                    "user_id": user_id,
                    "end": true,
                    "intention": "checkAutoStatus",
                    "message_id": crypto.randomUUID(),
                    "messages": {
                        "content": reply,
                        "attachments": []
                    }
                });
            } else {
                sendMessage(socket, 2001, {
                    "event_type": 2001,
                    "dialog_id": dialog_id,
                    "user_id": user_id,
                    "end": true,
                    "intention": "checkAutoStatus",
                    "message_id": crypto.randomUUID(),
                    "messages": {
                        "content": `查询 ${repoUrl} ${currentTag} 的自动构建状态完成: ${data.result}`,
                        "attachments": []
                    }
                });
            }

        } catch (error) {
            sendMessage(socket, 2001, {
                "event_type": 2001,
                "dialog_id": dialog_id,
                "user_id": user_id,
                "end": true,
                "intention": "checkAutoStatus",
                "message_id": crypto.randomUUID(),
                "messages": {
                    "content": `调用接口查询 ${repoUrl} ${currentTag} 的自动构建状态时出错: ${error.message}`,
                    "attachments": []
                }
            });
        } finally {
            // Reset query parameters
            repoUrl = null;
            tag = null;
            // End the conversation
            // setTimeout(() => {
            //     sendMessage(socket, 2002, { event_type: 2002, dialog_id, user_id });
            // }, 1000);
        }
    }

    /**
     * 处理用户的确认回复
     */
    async function handleConfirmation(userReply, dialog_id, user_id) {
        // 重置等待状态
        waitingForConfirmation = false;
        const tempRepoUrl = pendingRepoUrl;
        const tempTag = pendingTag;
        // 清空待确认信息
        pendingRepoUrl = null;
        pendingTag = null;

        // 定义关键词
        const denyKeywords = ['不', '不是', '不对', '错误', '不正确'];
        const confirmKeywords = ['是', '确认', '对', '没错', '正确', '嗯'];

        // 检查是否否认
        const isDenied = denyKeywords.some(kw => userReply.includes(kw));
        // 检查是否确认
        const isConfirmed = confirmKeywords.some(kw => userReply.includes(kw));

        // 如果明确确认且没有否认意图，则执行查询
        if (isConfirmed && !isDenied) {
            await performQuery(tempRepoUrl, tempTag, dialog_id, user_id);
        } else {
            // 否则，视为否认或提供了新信息，要求重新输入
            sendMessage(socket, 2001, {
                "event_type": 2001,
                "dialog_id": dialog_id,
                "user_id": user_id,
                "intention": "checkAutoStatus",
                "message_id": crypto.randomUUID(),
                "messages": {
                    "content": "好的，请提供新的发布仓地址和tag，例如: https://git.ipanel.cn/git/playcube/playcube.release.git   alpha-v1.0",
                    "attachments": []
                }
            });
            // 重置当前信息，准备接收新数据
            repoUrl = null;
            tag = null;
        }
    }


    socket.on('data', async (data) => {
        buffer = Buffer.concat([buffer, data]);

        while (buffer.length >= HEADER_LENGTH) {
            const header = buffer.slice(0, HEADER_LENGTH);
            const magic = header.readUInt32BE(0);
            if (magic !== 0xDEADBEEF) {
                console.error(`Invalid magic number: ${magic.toString(16)}. Closing connection.`);
                socket.end();
                return;
            }

            const bodyLength = header.readUInt32BE(8);
            const totalLength = HEADER_LENGTH + bodyLength;

            if (buffer.length < totalLength) {
                break; // Wait for more data
            }

            const bodyBuffer = buffer.slice(HEADER_LENGTH, totalLength);
            buffer = buffer.slice(totalLength);

            let message;
            try {
                message = JSON.parse(bodyBuffer.toString());
                console.log('Received message:', JSON.stringify(message, null, 2));
            } catch (e) {
                console.error('Error parsing incoming JSON:', e);
                return;
            }


            if (message.event_type === 1001) {
                const { dialog_id, user_id, messages: { content } } = message;

                if (content.includes('退出当前流程')) {
                    waitingForConfirmation = false;
                    pendingRepoUrl = null;
                    pendingTag = null;
                    repoUrl = null;
                    tag = null;
                    sendMessage(socket, 2002, {
                        "event_type": 2002,
                        "dialog_id": dialog_id,
                        "user_id": user_id,
                        "intention": "checkAutoStatus",
                        "message_id": crypto.randomUUID(),
                        "messages": {
                            "content": "好的，已退出当前流程。",
                            "attachments": []
                        }
                    });
                    return;
                }

                // 检查是否处于等待用户确认的状态
                if (waitingForConfirmation) {
                    await handleConfirmation(content, dialog_id, user_id);
                    return;
                }

                // 解析URL
                const urlRegex =  /(https?:\/\/[^\s]+\.release\.git)/;
                const urlMatch = content.match(urlRegex);
                if (urlMatch) {
                    repoUrl = urlMatch[0];
                }

                // 解析标签
                const tagRegex = /([a-zA-Z0-9]+-v\d+\.\d+|v\d+\.\d+)/;
                const messageWithoutUrl = content.replace(urlRegex, '');
                const tagMatch = messageWithoutUrl.match(tagRegex);
                if (tagMatch) {
                    tag = tagMatch[0];
                }

                const keywords = [
                    '刚刚', '刚才', '上次', '上一个', '上一次',
                    '之前', '前边', '前面', '先前', '刚才的',
                    '上次的', '之前的', '前边的', '前面的',
                    '方才', '适才', '刚才能', '方才的', '适才的',
                    '最近一次', '上回', '上回的'
                ];
                const shouldUsePrevious = keywords.some(kw => content.includes(kw));

                if (shouldUsePrevious) {
                    if (!repoUrl) {
                        try {
                            const mem = await queryMemory(port, "repoUrl", user_id);
                            if (mem && mem.value) repoUrl = mem.value;
                        } catch (e) {
                            console.error("Could not query repoUrl from memory", e);
                        }
                    }
                    if (!tag) {
                        try {
                            const mem = await queryMemory(port, "tag", user_id);
                            if (mem && mem.value) tag = mem.value;
                        } catch(e) {
                            console.error("Could not query tag from memory", e);
                        }
                    }

                    // 检查是否成功获取到历史数据
                    if (!repoUrl || !tag) {
                        sendMessage(socket, 2001, {
                            "event_type": 2001,
                            "dialog_id": dialog_id,
                            "user_id": user_id,
                            "intention": "checkAutoStatus",
                            "message_id": crypto.randomUUID(),
                            "messages": {
                                "content": "未找到历史查询记录，请提供发布仓地址和tag",
                                "attachments": []
                            }
                        });
                        return;
                    }

                    // 存储待确认的信息并进入等待确认状态
                    pendingRepoUrl = repoUrl;
                    pendingTag = tag;
                    waitingForConfirmation = true;

                    sendMessage(socket, 2001, {
                        "event_type": 2001,
                        "dialog_id": dialog_id,
                        "user_id": user_id,
                        "intention": "checkAutoStatus",
                        "message_id": crypto.randomUUID(),
                        "messages": {
                            "content": `请确认发布仓地址和tag是否正确：\n${repoUrl}\n${tag}\n请回复"是"或"确认"继续，或直接输入新的地址和tag进行修改`,
                            "attachments": []
                        }
                    });
                    return;
                }

                // 检查信息是否缺失并询问
                if (!repoUrl && !tag) {
                    sendMessage(socket, 2001, {
                        "event_type": 2001,
                        "dialog_id": dialog_id,
                        "user_id": user_id,
                        "intention": "checkAutoStatus",
                        "message_id": crypto.randomUUID(),
                        "messages": {
                            "content": '请提供发布仓的地址和发布tag，例如: https://git.ipanel.cn/git/playcube/playcube.release.git   alpha-v1.0',
                            "attachments": []
                        }
                    });
                    return;
                } else if (!repoUrl) {
                    sendMessage(socket, 2001, {
                        "event_type": 2001,
                        "dialog_id": dialog_id,
                        "user_id": user_id,
                        "intention": "checkAutoStatus",
                        "message_id": crypto.randomUUID(),
                        "messages": {
                            "content": '请提供发布仓的地址，例如: https://git.ipanel.cn/git/playcube/playcube.release.git',
                            "attachments": []
                        }
                    });
                    return;
                } else if (!tag) {
                    sendMessage(socket, 2001, {
                        "event_type": 2001,
                        "dialog_id": dialog_id,
                        "user_id": user_id,
                        "intention": "checkAutoStatus",
                        "message_id": crypto.randomUUID(),
                        "messages": {
                            "content": '请提供发布tag，例如: develop-v1.0',
                            "attachments": []
                        }
                    });
                    return;
                }

                // 信息完整，直接查询
                await performQuery(repoUrl, tag, dialog_id, user_id);

            } else if (message.event_type === 1002) {
                console.log('Received termination signal. Exiting.');
                socket.end();
            }
        }
    });

    socket.on('end', () => {
        console.log('Disconnected from Go program.');
        process.exit(0);
    });

    socket.on('error', (err) => {
        console.error('Socket error:', err.message);
        process.exit(1);
    });
}


main();