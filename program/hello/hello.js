const net = require('net');
const http = require('http');

/**
 * Parses command line arguments in the format --key=value
 * @returns {object} An object containing the parsed arguments.
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
 * Creates and sends a message over the socket according to the specified protocol.
 * @param {net.Socket} socket The socket to write the message to.
 * @param {number} eventType The event type for the message.
 * @param {object} message The JSON payload.
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
 * Queries the memory store via the HTTP interface provided by the Go host.
 * @param {number} port The port of the HTTP server.
 * @param {string} key The key to query.
 * @param {string} userId The user ID.
 * @returns {Promise<object>} A promise that resolves with the query result.
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
 * Saves a value to the memory store via the HTTP interface.
 * @param {number} port The port of the HTTP server.
 * @param {string} key The key to save.
 * @param {string} value The value to save.
 * @param {string} userId The user ID.
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
 * Main function to run the program.
 */
function main() {
    const args = parseArgs();
    const socketPath = args.socket;
    const port = args.port;

    if (!socketPath) {
        console.error('Error: Socket path not provided. Use --socket=/path/to/socket');
        process.exit(1);
    }

    console.log(`Attempting to connect to socket at: ${socketPath}`);
    const socket = net.createConnection({ path: socketPath });

    socket.on('connect', () => {
        console.log('Successfully connected to Go program via socket.');
    });

    let buffer = Buffer.alloc(0);
    const HEADER_LENGTH = 16;

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
                const { dialog_id, user_id, messages } = message;

                try {
                    await saveMemory(port, 'test', messages.content, user_id);
                    const memoryResponse = await queryMemory(port, 'test', user_id);

                    let content = `Hello from Node.js! You said: "${messages.content}".`;
                    if (memoryResponse && memoryResponse.value) {
                        content += ` I also know that the last release repo is: ${memoryResponse.value}`;
                    }

                    sendMessage(socket, 2001, {
                        event_type: 2001,
                        dialog_id,
                        user_id,
                        message_id: `msg-${Date.now()}`,
                        messages: { content }
                    });

                } catch (error) {
                    console.error("Error interacting with memory API:", error);
                    sendMessage(socket, 2001, {
                        event_type: 2001,
                        dialog_id,
                        user_id,
                        message_id: `msg-${Date.now()}`,
                        messages: { content: `Sorry, I had an error talking to my memory: ${error.message}` }
                    });
                }

                // End the conversation after a short delay
                setTimeout(() => {
                    sendMessage(socket, 2002, { event_type: 2002, dialog_id, user_id });
                    setTimeout(() => socket.end(), 500); // Close socket after sending final message
                }, 1000);

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
