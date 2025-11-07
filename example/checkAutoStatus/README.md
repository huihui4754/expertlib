# 获取自动构建状态技能

按照expertskill 中的定义实现和用户的聊天接口，在和用户聊天的接口中获取用户的发布仓地址和tag ，如果用户没有说这两个信息，需要提示用户说出这些信息，有了这些信息后需要调用接口来获取自动构建状态，提取其中信息返回给用户



## 接口示例 (GET)

```
 https://git.ipanel.cn/git/playcube/playcube.release.git/build/get_auto_info/develop-v1.0  
```
其中 https://git.ipanel.cn/git/playcube/playcube.release.git 为仓库地址，develop-v1.0 为tag。


回复示例

在线
```
{"reply":"ok","error_code":0,"data":{"auto名称":"9f2ff51_buildee","buildee名称":"192.168.18.113:18100","auto启动时间":"2025-08-19 04:01:07","健康状况":"健康","健康持续时长":"6时43分39秒","健康开始时间":"2025-08-19 04:02:34"}}
```

不在线
```  
{"error_code":60105,"reply":"failed","result":"编译任务不在线,无法获取健康状态信息."}

```