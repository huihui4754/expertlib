const CheckAutoStatus = require('./CheckAutoStatus');
const axios = require('axios');

// Mock axios
jest.mock('axios');

describe('CheckAutoStatus', () => {
    let skill;
    let websocketMessage;

    beforeEach(() => {
        skill = new CheckAutoStatus();
        websocketMessage = {
            dialog_id: 'test-dialog-id',
            user_id: 'test-user-id',
            messages: {
                content: ''
            }
        };
    });

    afterEach(() => {
        jest.clearAllMocks();
    });

    test('getSkillDescription 应返回正确的描述', () => {
        expect(skill.getSkillDescription()).toBe('获取自动构建状态');
    });

    test('如果未提供，应询问git 发布仓 URL', async () => {
        websocketMessage.messages.content = 'check status for v1.0';
        const response = await skill.chat(websocketMessage);
        expect(response.messages.content).toContain('请提供发布仓的地址');
    });

    test('如果未提供，应要求提供 tag', async () => {
        websocketMessage.messages.content = '检查这个发布仓的状态 https://git.ipanel.cn/git/playcube/playcube.release.git';
        const response = await skill.chat(websocketMessage);
        expect(response.messages.content).toContain('请提供发布tag');
    });

    test('应处理成功API调用', async () => {
        const repoUrl = 'https://git.ipanel.cn/git/playcube/playcube.release.git';
        const tag = 'develop-v1.0';
        websocketMessage.messages.content = `check status for ${repoUrl} with tag ${tag}`;

        const mockResponse = {
            data: {
                error_code: 0,
                data: {
                    'auto名称': 'test-auto',
                    'buildee名称': 'test-buildee',
                    'auto启动时间': '2025-08-19 10:00:00',
                    '健康状况': 'healthy',
                    '健康持续时长': '1 hour',
                    '健康开始时间': '2025-08-19 09:00:00'
                }
            }
        };
        axios.get.mockResolvedValue(mockResponse);

        const response = await skill.chat(websocketMessage);
        expect(axios.get).toHaveBeenCalledWith(`${repoUrl}/build/get_auto_info/${tag}`, expect.anything());
        expect(response.messages.content).toContain(`查询 ${repoUrl} 的 ${tag} 成功`);
        expect(response.messages.content).toContain('Auto名称: test-auto');
    });

    test('should handle API call with error_code not 0', async () => {
        const repoUrl = 'https://git.ipanel.cn/git/playcube/playcube.release.git';
        const tag = 'develop-v1.0';
        websocketMessage.messages.content = `check status for ${repoUrl} with tag ${tag}`;

        const mockResponse = {
            data: {
                error_code: 1,
                result: 'some error'
            }
        };
        axios.get.mockResolvedValue(mockResponse);

        const response = await skill.chat(websocketMessage);
        expect(axios.get).toHaveBeenCalledWith(`${repoUrl}/build/get_auto_info/${tag}`, expect.anything());
        expect(response.messages.content).toContain(`查询 ${repoUrl} 的 ${tag} 完成: some error`);
    });

    test('should handle API call failure', async () => {
        const repoUrl = 'https://git.ipanel.cn/git/playcube/playcube.release.git';
        const tag = 'develop-v1.0';
        websocketMessage.messages.content = `check status for ${repoUrl} with tag ${tag}`;

        const errorMessage = 'Network Error';
        axios.get.mockRejectedValue(new Error(errorMessage));

        const response = await skill.chat(websocketMessage);
        expect(axios.get).toHaveBeenCalledWith(`${repoUrl}/build/get_auto_info/${tag}`, expect.anything());
        expect(response.messages.content).toContain(`调用接口查询 ${repoUrl} 的 ${tag} 时出错: ${errorMessage}`);
    });
});