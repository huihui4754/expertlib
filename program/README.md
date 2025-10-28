## program 程序库接口

```go

NewTool() *Tool  // 获取program 实例
 
(t *Tool) SetDataFilePath(string) // 设置程序库保存文件路径  不设置默认用 ~/expert/program/  支持配置文件设置
(t *Tool) SetProgramPath(string) // 设置本地js 程序库路径 不设置默认用 ~/expert/js/  支持配置文件设置

(t *Tool) HandleExpertRequestMessage(jsonx any)  // 给程序库的消息由此传入,传入是一个消息对象
(t *Tool) HandleExpertRequestMessageString(string) // 给程序库的消息由此传入,传入的是一个字符串，tools 内部会自己解析，和 HandleExpertRequestMessage 二选一使用
(t *Tool) SetToExpertMessageHandler(func(any,string))  // 由此监听程序库返回的消息

(t *Tool) Run() // 启动程序库实例

(t *Tool) GetProgramName() []string // 获取程序库所有的程序的名称
(t *Tool) UpdateProgram()  // 从js path 重新加载程序库 

```