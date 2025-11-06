## program 程序库接口

```go

NewTool() *Tool  // 获取program 实例
 
(t *Tool) SetDataFilePath(string) // 设置程序库保存文件路径  不设置默认用 ~/expert/program/  支持配置文件设置
(t *Tool) SetProgramPath(string) // 设置本地js 程序库路径 不设置默认用 ~/expert/js/  支持配置文件设置

(t *Tool) HandleExpertRequestMessage(any)  // 给程序库的消息由此传入，支持 TotalMessage ， string ,[]byte 等多种类型
(t *Tool) SetToExpertMessageHandler(func(TotalMessage,string))  // 由此监听程序库返回的消息

(t *Tool) Run() // 启动程序库实例

(t *Tool) GetProgramNames() []string // 获取程序库所有的程序的名称
(t *Tool) UpdatePrograms()  // 从js path 重新加载程序库 

```